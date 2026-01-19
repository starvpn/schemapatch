package extractor

import (
	"context"

	"github.com/schemapatch/schemapatch/internal/config"
)

// ExtractOptions 提取选项
type ExtractOptions struct {
	IncludeTables     bool     // 是否包含表
	IncludeViews      bool     // 是否包含视图
	IncludeProcedures bool     // 是否包含存储过程
	IncludeFunctions  bool     // 是否包含函数
	IncludeTriggers   bool     // 是否包含触发器
	TableFilter       []string // 只提取这些表（为空则提取全部）
	ExcludeTables     []string // 排除这些表
}

// DefaultExtractOptions 默认提取选项
func DefaultExtractOptions() ExtractOptions {
	return ExtractOptions{
		IncludeTables:     true,
		IncludeViews:      true,
		IncludeProcedures: true,
		IncludeFunctions:  true,
		IncludeTriggers:   true,
	}
}

// SchemaExtractor Schema提取器接口
type SchemaExtractor interface {
	// Connect 连接数据库
	Connect(ctx context.Context) error

	// Close 关闭连接
	Close() error

	// ExtractSchema 提取完整Schema
	ExtractSchema(ctx context.Context, options ExtractOptions) (*DatabaseSchema, error)

	// ExtractTables 只提取表结构
	ExtractTables(ctx context.Context, tableNames ...string) (map[string]*TableSchema, error)

	// ExtractViews 只提取视图
	ExtractViews(ctx context.Context) (map[string]*ViewSchema, error)

	// ExtractProcedures 只提取存储过程
	ExtractProcedures(ctx context.Context) (map[string]*ProcedureSchema, error)

	// ExtractFunctions 只提取函数
	ExtractFunctions(ctx context.Context) (map[string]*FunctionSchema, error)

	// ExtractTriggers 只提取触发器
	ExtractTriggers(ctx context.Context) (map[string]*TriggerSchema, error)

	// GetServerVersion 获取数据库版本
	GetServerVersion(ctx context.Context) (string, error)

	// GetServerVariables 获取服务器变量
	GetServerVariables(ctx context.Context) (map[string]string, error)

	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// NewExtractor 根据环境配置创建提取器
func NewExtractor(env *config.Environment) (SchemaExtractor, error) {
	return NewMySQLExtractor(env)
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(current, total int, message string)

// ExtractWithProgress 带进度回调的提取
func ExtractWithProgress(ctx context.Context, extractor SchemaExtractor, options ExtractOptions, callback ProgressCallback) (*DatabaseSchema, error) {
	schema := NewDatabaseSchema("")
	totalSteps := 0
	currentStep := 0

	// 计算总步骤数
	if options.IncludeTables {
		totalSteps++
	}
	if options.IncludeViews {
		totalSteps++
	}
	if options.IncludeProcedures {
		totalSteps++
	}
	if options.IncludeFunctions {
		totalSteps++
	}
	if options.IncludeTriggers {
		totalSteps++
	}

	// 提取表
	if options.IncludeTables {
		currentStep++
		if callback != nil {
			callback(currentStep, totalSteps, "正在提取表结构...")
		}
		tables, err := extractor.ExtractTables(ctx)
		if err != nil {
			return nil, err
		}
		schema.Tables = tables
	}

	// 提取视图
	if options.IncludeViews {
		currentStep++
		if callback != nil {
			callback(currentStep, totalSteps, "正在提取视图...")
		}
		views, err := extractor.ExtractViews(ctx)
		if err != nil {
			return nil, err
		}
		schema.Views = views
	}

	// 提取存储过程
	if options.IncludeProcedures {
		currentStep++
		if callback != nil {
			callback(currentStep, totalSteps, "正在提取存储过程...")
		}
		procedures, err := extractor.ExtractProcedures(ctx)
		if err != nil {
			return nil, err
		}
		schema.Procedures = procedures
	}

	// 提取函数
	if options.IncludeFunctions {
		currentStep++
		if callback != nil {
			callback(currentStep, totalSteps, "正在提取函数...")
		}
		functions, err := extractor.ExtractFunctions(ctx)
		if err != nil {
			return nil, err
		}
		schema.Functions = functions
	}

	// 提取触发器
	if options.IncludeTriggers {
		currentStep++
		if callback != nil {
			callback(currentStep, totalSteps, "正在提取触发器...")
		}
		triggers, err := extractor.ExtractTriggers(ctx)
		if err != nil {
			return nil, err
		}
		schema.Triggers = triggers
	}

	return schema, nil
}
