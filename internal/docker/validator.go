package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/starvpn/schemapatch/internal/config"
	"github.com/starvpn/schemapatch/internal/extractor"
	"github.com/starvpn/schemapatch/internal/sqlgen"
	"go.uber.org/zap"
)

// ValidationOptions 验证选项
type ValidationOptions struct {
	MySQLImage     string        // MySQL镜像
	Timeout        time.Duration // 超时时间
	Cleanup        bool          // 验证后是否清理
	QuickMode      bool          // 快速模式（仅语法检查）
	CompareSchema  bool          // 验证后对比Schema
}

// DefaultValidationOptions 默认验证选项
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		MySQLImage:    "mysql:8.0",
		Timeout:       120 * time.Second,
		Cleanup:       true,
		QuickMode:     false,
		CompareSchema: true,
	}
}

// ValidationResult 验证结果
type ValidationResult struct {
	Success       bool                 `json:"success"`
	ExecutionLog  []ExecutionLogEntry  `json:"execution_log"`
	Errors        []string             `json:"errors"`
	Warnings      []string             `json:"warnings"`
	SchemaMatch   bool                 `json:"schema_match"`
	SchemaDiffs   []string             `json:"schema_diffs,omitempty"`
	ExecutionTime time.Duration        `json:"execution_time"`
	ContainerLog  string               `json:"container_log"`
}

// ExecutionLogEntry 执行日志条目
type ExecutionLogEntry struct {
	Timestamp time.Time     `json:"timestamp"`
	Step      int           `json:"step"`
	Total     int           `json:"total"`
	Message   string        `json:"message"`
	SQL       string        `json:"sql,omitempty"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// ProgressCallback 进度回调
type ProgressCallback func(step int, total int, message string, err error)

// Validator 验证器
type Validator struct {
	manager *Manager
}

// NewValidator 创建验证器
func NewValidator() *Validator {
	return &Validator{
		manager: NewManager(),
	}
}

// Validate 验证迁移脚本
// sourceSchema: 开发环境Schema（升级目标状态）
// targetSchema: 生产环境Schema（当前状态，需要被升级）
func (v *Validator) Validate(ctx context.Context, sourceSchema, targetSchema *extractor.DatabaseSchema, script *sqlgen.MigrationScript, options ValidationOptions, callback ProgressCallback) (*ValidationResult, error) {
	startTime := time.Now()

	result := &ValidationResult{
		Success:      false,
		ExecutionLog: []ExecutionLogEntry{},
		Errors:       []string{},
		Warnings:     []string{},
		SchemaMatch:  false,
	}

	// 计算总步骤数
	totalSteps := 4 + len(script.Statements) // 检查Docker + 创建容器 + 等待就绪 + 执行语句 + 清理
	currentStep := 0

	// 步骤1: 检查Docker
	currentStep++
	v.logStep(result, currentStep, totalSteps, "检查Docker环境...", "", true, nil)
	if callback != nil {
		callback(currentStep, totalSteps, "检查Docker环境...", nil)
	}

	if err := v.manager.CheckDockerAvailable(ctx); err != nil {
		v.logStep(result, currentStep, totalSteps, "Docker检查失败", "", false, err)
		result.Errors = append(result.Errors, "Docker不可用: "+err.Error())
		return result, err
	}

	// 步骤2: 创建容器
	currentStep++
	v.logStep(result, currentStep, totalSteps, "创建MySQL容器...", "", true, nil)
	if callback != nil {
		callback(currentStep, totalSteps, "创建MySQL容器...", nil)
	}

	containerConfig := ContainerConfig{
		MySQLImage:   options.MySQLImage,
		RootPassword: "schemapatch_test",
		Database:     "test_db",
		Charset:      targetSchema.Charset,
		Collation:    targetSchema.Collation,
		Timeout:      options.Timeout,
	}

	container, err := v.manager.CreateContainer(ctx, containerConfig)
	if err != nil {
		v.logStep(result, currentStep, totalSteps, "创建容器失败", "", false, err)
		result.Errors = append(result.Errors, "创建容器失败: "+err.Error())
		return result, err
	}

	// 确保清理
	if options.Cleanup {
		defer v.manager.RemoveContainer(ctx, container.ID)
	}

	// 步骤3: 等待MySQL就绪
	currentStep++
	v.logStep(result, currentStep, totalSteps, "等待MySQL就绪...", "", true, nil)
	if callback != nil {
		callback(currentStep, totalSteps, "等待MySQL就绪...", nil)
	}

	if err := v.manager.WaitForMySQL(ctx, container); err != nil {
		v.logStep(result, currentStep, totalSteps, "MySQL启动超时", "", false, err)
		result.Errors = append(result.Errors, "MySQL启动超时: "+err.Error())
		result.ContainerLog, _ = v.manager.GetContainerLogs(ctx, container.ID, 100)
		return result, err
	}

	// 步骤4: 导入目标Schema（生产环境当前状态）
	currentStep++
	v.logStep(result, currentStep, totalSteps, "导入目标Schema...", "", true, nil)
	if callback != nil {
		callback(currentStep, totalSteps, "导入目标Schema...", nil)
	}

	if err := v.importSchema(ctx, container, targetSchema); err != nil {
		v.logStep(result, currentStep, totalSteps, "导入Schema失败", "", false, err)
		result.Errors = append(result.Errors, "导入Schema失败: "+err.Error())
		return result, err
	}

	// 步骤5-N: 执行升级语句
	executeStart := time.Now()
	successCount := 0
	failCount := 0

	for i, stmt := range script.Statements {
		currentStep++
		stepMsg := fmt.Sprintf("执行 [%d/%d]: %s.%s", i+1, len(script.Statements), stmt.Operation, stmt.ObjectName)

		if callback != nil {
			callback(currentStep, totalSteps, stepMsg, nil)
		}

		var execResult *ExecutionResult
		var execErr error

		// 对于触发器、存储过程、函数，使用分隔符执行（它们可能包含多个分号）
		if stmt.ObjectType == "TRIGGER" || stmt.ObjectType == "PROCEDURE" || stmt.ObjectType == "FUNCTION" {
			// 去掉末尾的分号（如果有），然后用 $$ 作为分隔符
			sql := strings.TrimSuffix(strings.TrimSpace(stmt.SQL), ";")
			execResult, execErr = v.manager.ExecuteSQLWithDelimiter(ctx, container, sql+"\n$$", "$$")
		} else {
			execResult, execErr = v.manager.ExecuteSQL(ctx, container, stmt.SQL)
		}

		if execErr != nil || !execResult.Success {
			failCount++
			errMsg := ""
			if execErr != nil {
				errMsg = execErr.Error()
			} else {
				errMsg = execResult.Error
			}
			v.logStep(result, currentStep, totalSteps, stepMsg, stmt.SQL, false, fmt.Errorf(errMsg))
			result.Errors = append(result.Errors, fmt.Sprintf("语句 %d 执行失败: %s", i+1, errMsg))

			if callback != nil {
				callback(currentStep, totalSteps, stepMsg, fmt.Errorf(errMsg))
			}
		} else {
			successCount++
			v.logStep(result, currentStep, totalSteps, stepMsg+" ✓", stmt.SQL, true, nil)
		}
	}

	executeDuration := time.Since(executeStart)
	zap.S().Infof("执行完成: 成功 %d, 失败 %d, 耗时 %v", successCount, failCount, executeDuration)

	// 验证Schema一致性（比较升级后的结果与开发环境Schema）
	if options.CompareSchema && failCount == 0 {
		currentStep++
		v.logStep(result, currentStep, totalSteps, "验证Schema一致性（对比开发环境）...", "", true, nil)
		if callback != nil {
			callback(currentStep, totalSteps, "验证Schema一致性...", nil)
		}

		// 比较容器中升级后的Schema与开发环境（sourceSchema）是否一致
		match, diffs := v.compareSchemaInContainer(ctx, container, sourceSchema)
		result.SchemaMatch = match
		result.SchemaDiffs = diffs

		if !match {
			result.Warnings = append(result.Warnings, "升级后Schema与开发环境仍有差异")
		}
	}

	// 获取容器日志
	result.ContainerLog, _ = v.manager.GetContainerLogs(ctx, container.ID, 50)

	// 设置结果
	result.Success = failCount == 0
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// importSchema 导入Schema到容器
func (v *Validator) importSchema(ctx context.Context, container *Container, schema *extractor.DatabaseSchema) error {
	// 第一步：导入表和视图（普通SQL，用分号分隔）
	var sqlBuilder strings.Builder

	// 设置字符集
	sqlBuilder.WriteString(fmt.Sprintf("SET NAMES '%s';\n", schema.Charset))
	sqlBuilder.WriteString("SET FOREIGN_KEY_CHECKS = 0;\n\n")

	// 创建表
	for _, table := range schema.Tables {
		if table.CreateSQL != "" {
			sqlBuilder.WriteString(table.CreateSQL)
			sqlBuilder.WriteString(";\n\n")
		}
	}

	// 创建视图
	for name, view := range schema.Views {
		if view.Definition != "" {
			sqlBuilder.WriteString(fmt.Sprintf("CREATE VIEW `%s` AS %s;\n\n", name, view.Definition))
		}
	}

	sqlBuilder.WriteString("SET FOREIGN_KEY_CHECKS = 1;\n")

	// 执行表和视图的SQL
	result, err := v.manager.ExecuteSQL(ctx, container, sqlBuilder.String())
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("导入Schema失败: %s", result.Error)
	}

	// 第二步：单独导入存储过程（使用 $$ 作为分隔符）
	for name, proc := range schema.Procedures {
		if proc.Definition != "" {
			if err := v.importRoutine(ctx, container, "PROCEDURE", name, proc.Definition); err != nil {
				return fmt.Errorf("导入存储过程 %s 失败: %w", name, err)
			}
		}
	}

	// 第三步：单独导入函数
	for name, fn := range schema.Functions {
		if fn.Definition != "" {
			if err := v.importRoutine(ctx, container, "FUNCTION", name, fn.Definition); err != nil {
				return fmt.Errorf("导入函数 %s 失败: %w", name, err)
			}
		}
	}

	// 第四步：导入触发器
	for _, trigger := range schema.Triggers {
		triggerSQL := fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW %s",
			trigger.Name, trigger.Timing, trigger.Event, trigger.Table, trigger.Statement)
		if err := v.importRoutine(ctx, container, "TRIGGER", trigger.Name, triggerSQL); err != nil {
			return fmt.Errorf("导入触发器 %s 失败: %w", trigger.Name, err)
		}
	}

	return nil
}

// importRoutine 导入存储过程/函数/触发器（使用自定义分隔符）
func (v *Validator) importRoutine(ctx context.Context, container *Container, routineType, name, definition string) error {
	// 使用 $$ 作为分隔符来执行包含分号的语句
	sql := definition + "\n$$"

	result, err := v.manager.ExecuteSQLWithDelimiter(ctx, container, sql, "$$")
	if err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("%s", result.Error)
	}

	zap.S().Debugf("导入%s成功: %s", routineType, name)
	return nil
}

// compareSchemaInContainer 在容器中比较Schema
func (v *Validator) compareSchemaInContainer(ctx context.Context, container *Container, expectedSchema *extractor.DatabaseSchema) (bool, []string) {
	// 创建提取器连接到容器
	env := &config.Environment{
		Host:     container.Host,
		Port:     container.Port,
		Username: "root",
		Password: container.Config.RootPassword,
		Database: container.Config.Database,
		Charset:  container.Config.Charset,
	}

	ext, err := extractor.NewMySQLExtractor(env)
	if err != nil {
		return false, []string{"创建提取器失败: " + err.Error()}
	}
	defer ext.Close()

	if err := ext.Connect(ctx); err != nil {
		return false, []string{"连接容器数据库失败: " + err.Error()}
	}

	// 提取当前Schema
	opts := extractor.DefaultExtractOptions()
	currentSchema, err := ext.ExtractSchema(ctx, opts)
	if err != nil {
		return false, []string{"提取Schema失败: " + err.Error()}
	}

	// 简单比较表数量
	var diffs []string

	if len(currentSchema.Tables) != len(expectedSchema.Tables) {
		diffs = append(diffs, fmt.Sprintf("表数量不匹配: 期望 %d, 实际 %d",
			len(expectedSchema.Tables), len(currentSchema.Tables)))
	}

	// 检查每个表
	for tableName := range expectedSchema.Tables {
		if _, exists := currentSchema.Tables[tableName]; !exists {
			diffs = append(diffs, fmt.Sprintf("缺少表: %s", tableName))
		}
	}

	return len(diffs) == 0, diffs
}

// logStep 记录步骤日志
func (v *Validator) logStep(result *ValidationResult, step, total int, message, sql string, success bool, err error) {
	entry := ExecutionLogEntry{
		Timestamp: time.Now(),
		Step:      step,
		Total:     total,
		Message:   message,
		SQL:       sql,
		Success:   success,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	result.ExecutionLog = append(result.ExecutionLog, entry)

	if success {
		zap.S().Infof("[%d/%d] %s", step, total, message)
	} else {
		zap.S().Errorf("[%d/%d] %s: %v", step, total, message, err)
	}
}

// QuickValidate 快速验证（仅语法检查）
func (v *Validator) QuickValidate(ctx context.Context, script *sqlgen.MigrationScript) (*ValidationResult, error) {
	result := &ValidationResult{
		Success:      true,
		ExecutionLog: []ExecutionLogEntry{},
		Errors:       []string{},
		Warnings:     []string{},
	}

	// 基本语法检查
	for i, stmt := range script.Statements {
		sql := strings.TrimSpace(stmt.SQL)

		// 检查空语句
		if sql == "" || sql == ";" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("语句 %d 为空", i+1))
			continue
		}

		// 检查基本语法
		if !strings.HasSuffix(sql, ";") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("语句 %d 缺少分号", i+1))
		}

		// 检查危险操作
		upperSQL := strings.ToUpper(sql)
		if strings.Contains(upperSQL, "DROP TABLE") && !strings.Contains(upperSQL, "IF EXISTS") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("语句 %d: DROP TABLE 建议使用 IF EXISTS", i+1))
		}
		if strings.Contains(upperSQL, "TRUNCATE") {
			result.Warnings = append(result.Warnings, fmt.Sprintf("语句 %d: 包含 TRUNCATE 语句", i+1))
		}
	}

	return result, nil
}

// Cleanup 清理所有资源
func (v *Validator) Cleanup(ctx context.Context) {
	v.manager.Cleanup(ctx)
}
