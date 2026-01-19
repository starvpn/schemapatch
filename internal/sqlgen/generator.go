package sqlgen

import (
	"fmt"
	"strings"
	"time"

	"github.com/starvpn/schemapatch/internal/diff"
	"github.com/starvpn/schemapatch/internal/extractor"
)

// GenerateOptions 生成选项
type GenerateOptions struct {
	IncludeRollback  bool   // 是否生成回滚脚本
	WrapTransaction  bool   // 是否包装事务
	AddComments      bool   // 是否添加注释说明
	SafeMode         bool   // 安全模式（危险操作需确认）
	OnlineMode       bool   // 在线变更模式（大表友好）
	Delimiter        string // 语句分隔符
}

// DefaultGenerateOptions 默认生成选项
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		IncludeRollback: false,
		WrapTransaction: false,
		AddComments:     true,
		SafeMode:        true,
		OnlineMode:      false,
		Delimiter:       ";",
	}
}

// MigrationScript 迁移脚本
type MigrationScript struct {
	Version       string          `json:"version"`
	Description   string          `json:"description"`
	UpSQL         string          `json:"up_sql"`
	DownSQL       string          `json:"down_sql"`
	Statements    []SQLStatement  `json:"statements"`
	Warnings      []string        `json:"warnings"`
	EstimatedTime time.Duration   `json:"estimated_time"`
	GeneratedAt   time.Time       `json:"generated_at"`
}

// SQLStatement SQL语句
type SQLStatement struct {
	SQL        string            `json:"sql"`
	ObjectType string            `json:"object_type"` // TABLE, INDEX, VIEW...
	ObjectName string            `json:"object_name"`
	Operation  string            `json:"operation"`   // CREATE, ALTER, DROP
	Severity   diff.DiffSeverity `json:"severity"`
	Comment    string            `json:"comment"`
	RollbackSQL string           `json:"rollback_sql,omitempty"`
}

// SQLGenerator SQL生成器接口
type SQLGenerator interface {
	Generate(schemaDiff *diff.SchemaDiff, options GenerateOptions) (*MigrationScript, error)
}

// MySQLGenerator MySQL SQL生成器
type MySQLGenerator struct{}

// NewMySQLGenerator 创建MySQL生成器
func NewMySQLGenerator() *MySQLGenerator {
	return &MySQLGenerator{}
}

// Generate 生成迁移脚本
func (g *MySQLGenerator) Generate(schemaDiff *diff.SchemaDiff, options GenerateOptions) (*MigrationScript, error) {
	script := &MigrationScript{
		Version:     time.Now().Format("20060102150405"),
		Description: fmt.Sprintf("从 %s 迁移到 %s", schemaDiff.TargetEnv, schemaDiff.SourceEnv),
		Statements:  []SQLStatement{},
		Warnings:    []string{},
		GeneratedAt: time.Now(),
	}

	// 按依赖顺序生成SQL
	// 1. 先删除外键约束
	// 2. 删除触发器
	// 3. 删除视图、存储过程、函数
	// 4. 修改表结构（删除列、修改列）
	// 5. 创建新表
	// 6. 添加列
	// 7. 创建索引
	// 8. 创建外键
	// 9. 创建视图、存储过程、函数、触发器

	// 收集所有需要删除的外键
	var dropFKStatements []SQLStatement
	var dropTriggerStatements []SQLStatement
	var dropViewStatements []SQLStatement
	var dropProcStatements []SQLStatement
	var dropFuncStatements []SQLStatement
	var dropTableStatements []SQLStatement
	var alterTableStatements []SQLStatement
	var createTableStatements []SQLStatement
	var createIndexStatements []SQLStatement
	var createFKStatements []SQLStatement
	var createTriggerStatements []SQLStatement
	var createViewStatements []SQLStatement
	var createProcStatements []SQLStatement
	var createFuncStatements []SQLStatement

	// 处理表差异
	for _, td := range schemaDiff.TableDiffs {
		switch td.DiffType {
		case diff.DiffTypeAdded:
			// 新增表
			if td.NewTable != nil && td.NewTable.CreateSQL != "" {
				createTableStatements = append(createTableStatements, SQLStatement{
					SQL:        td.NewTable.CreateSQL + ";",
					ObjectType: "TABLE",
					ObjectName: td.TableName,
					Operation:  "CREATE",
					Severity:   diff.SeverityInfo,
					Comment:    "创建新表",
				})
			}

		case diff.DiffTypeRemoved:
			// 先删除外键
			if td.OldTable != nil {
				for fkName := range td.OldTable.ForeignKeys {
					dropFKStatements = append(dropFKStatements, SQLStatement{
						SQL:        fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`;", td.TableName, fkName),
						ObjectType: "FOREIGN KEY",
						ObjectName: fmt.Sprintf("%s.%s", td.TableName, fkName),
						Operation:  "DROP",
						Severity:   diff.SeverityWarning,
						Comment:    "删除外键约束",
					})
				}
			}

			// 删除表
			dropTableStatements = append(dropTableStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP TABLE IF EXISTS `%s`;", td.TableName),
				ObjectType: "TABLE",
				ObjectName: td.TableName,
				Operation:  "DROP",
				Severity:   diff.SeverityDanger,
				Comment:    "⚠️ 删除表 - 数据将丢失",
			})
			script.Warnings = append(script.Warnings,
				fmt.Sprintf("删除表 `%s` 将导致所有数据永久丢失", td.TableName))

		case diff.DiffTypeModified:
			// 处理外键变更
			for _, fkd := range td.FKeyDiffs {
				switch fkd.DiffType {
				case diff.DiffTypeRemoved:
					dropFKStatements = append(dropFKStatements, SQLStatement{
						SQL:        fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`;", td.TableName, fkd.FKeyName),
						ObjectType: "FOREIGN KEY",
						ObjectName: fmt.Sprintf("%s.%s", td.TableName, fkd.FKeyName),
						Operation:  "DROP",
						Severity:   diff.SeverityWarning,
						Comment:    "删除外键约束",
					})
				case diff.DiffTypeAdded:
					if fkd.NewFKey != nil {
						createFKStatements = append(createFKStatements, g.generateAddForeignKey(td.TableName, fkd.NewFKey))
					}
				case diff.DiffTypeModified:
					// 先删后加
					dropFKStatements = append(dropFKStatements, SQLStatement{
						SQL:        fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`;", td.TableName, fkd.FKeyName),
						ObjectType: "FOREIGN KEY",
						ObjectName: fmt.Sprintf("%s.%s", td.TableName, fkd.FKeyName),
						Operation:  "DROP",
						Severity:   diff.SeverityWarning,
						Comment:    "删除外键约束（将重建）",
					})
					if fkd.NewFKey != nil {
						createFKStatements = append(createFKStatements, g.generateAddForeignKey(td.TableName, fkd.NewFKey))
					}
				}
			}

			// 处理索引变更
			for _, id := range td.IndexDiffs {
				stmts := g.generateIndexStatements(td.TableName, &id)
				for _, stmt := range stmts {
					if stmt.Operation == "DROP" {
						alterTableStatements = append(alterTableStatements, stmt)
					} else {
						createIndexStatements = append(createIndexStatements, stmt)
					}
				}
			}

			// 处理列变更
			for _, cd := range td.ColumnDiffs {
				stmts := g.generateColumnStatements(td.TableName, &cd)
				alterTableStatements = append(alterTableStatements, stmts...)
			}

			// 处理表属性变更
			for _, prop := range td.TableProps {
				stmt := g.generateTablePropertyStatement(td.TableName, &prop)
				if stmt != nil {
					alterTableStatements = append(alterTableStatements, *stmt)
				}
			}
		}
	}

	// 处理视图差异
	for _, vd := range schemaDiff.ViewDiffs {
		switch vd.DiffType {
		case diff.DiffTypeAdded:
			if vd.NewView != nil {
				createViewStatements = append(createViewStatements, SQLStatement{
					SQL:        fmt.Sprintf("CREATE VIEW `%s` AS %s;", vd.ViewName, vd.NewView.Definition),
					ObjectType: "VIEW",
					ObjectName: vd.ViewName,
					Operation:  "CREATE",
					Severity:   diff.SeverityInfo,
					Comment:    "创建视图",
				})
			}
		case diff.DiffTypeRemoved:
			dropViewStatements = append(dropViewStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP VIEW IF EXISTS `%s`;", vd.ViewName),
				ObjectType: "VIEW",
				ObjectName: vd.ViewName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除视图",
			})
		case diff.DiffTypeModified:
			if vd.NewView != nil {
				createViewStatements = append(createViewStatements, SQLStatement{
					SQL:        fmt.Sprintf("CREATE OR REPLACE VIEW `%s` AS %s;", vd.ViewName, vd.NewView.Definition),
					ObjectType: "VIEW",
					ObjectName: vd.ViewName,
					Operation:  "ALTER",
					Severity:   diff.SeverityWarning,
					Comment:    "修改视图",
				})
			}
		}
	}

	// 处理存储过程差异
	for _, pd := range schemaDiff.ProcDiffs {
		switch pd.DiffType {
		case diff.DiffTypeAdded:
			if pd.NewProc != nil && pd.NewProc.Definition != "" {
				createProcStatements = append(createProcStatements, SQLStatement{
					SQL:        pd.NewProc.Definition + ";",
					ObjectType: "PROCEDURE",
					ObjectName: pd.ProcName,
					Operation:  "CREATE",
					Severity:   diff.SeverityInfo,
					Comment:    "创建存储过程",
				})
			}
		case diff.DiffTypeRemoved:
			dropProcStatements = append(dropProcStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP PROCEDURE IF EXISTS `%s`;", pd.ProcName),
				ObjectType: "PROCEDURE",
				ObjectName: pd.ProcName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除存储过程",
			})
		case diff.DiffTypeModified:
			dropProcStatements = append(dropProcStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP PROCEDURE IF EXISTS `%s`;", pd.ProcName),
				ObjectType: "PROCEDURE",
				ObjectName: pd.ProcName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除存储过程（将重建）",
			})
			if pd.NewProc != nil && pd.NewProc.Definition != "" {
				createProcStatements = append(createProcStatements, SQLStatement{
					SQL:        pd.NewProc.Definition + ";",
					ObjectType: "PROCEDURE",
					ObjectName: pd.ProcName,
					Operation:  "CREATE",
					Severity:   diff.SeverityWarning,
					Comment:    "重建存储过程",
				})
			}
		}
	}

	// 处理函数差异
	for _, fd := range schemaDiff.FuncDiffs {
		switch fd.DiffType {
		case diff.DiffTypeAdded:
			if fd.NewFunc != nil && fd.NewFunc.Definition != "" {
				createFuncStatements = append(createFuncStatements, SQLStatement{
					SQL:        fd.NewFunc.Definition + ";",
					ObjectType: "FUNCTION",
					ObjectName: fd.FuncName,
					Operation:  "CREATE",
					Severity:   diff.SeverityInfo,
					Comment:    "创建函数",
				})
			}
		case diff.DiffTypeRemoved:
			dropFuncStatements = append(dropFuncStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP FUNCTION IF EXISTS `%s`;", fd.FuncName),
				ObjectType: "FUNCTION",
				ObjectName: fd.FuncName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除函数",
			})
		case diff.DiffTypeModified:
			dropFuncStatements = append(dropFuncStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP FUNCTION IF EXISTS `%s`;", fd.FuncName),
				ObjectType: "FUNCTION",
				ObjectName: fd.FuncName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除函数（将重建）",
			})
			if fd.NewFunc != nil && fd.NewFunc.Definition != "" {
				createFuncStatements = append(createFuncStatements, SQLStatement{
					SQL:        fd.NewFunc.Definition + ";",
					ObjectType: "FUNCTION",
					ObjectName: fd.FuncName,
					Operation:  "CREATE",
					Severity:   diff.SeverityWarning,
					Comment:    "重建函数",
				})
			}
		}
	}

	// 处理触发器差异
	for _, td := range schemaDiff.TriggerDiffs {
		switch td.DiffType {
		case diff.DiffTypeAdded:
			if td.NewTrigger != nil {
				createTriggerStatements = append(createTriggerStatements, SQLStatement{
					SQL: fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW %s;",
						td.TriggerName, td.NewTrigger.Timing, td.NewTrigger.Event,
						td.NewTrigger.Table, td.NewTrigger.Statement),
					ObjectType: "TRIGGER",
					ObjectName: td.TriggerName,
					Operation:  "CREATE",
					Severity:   diff.SeverityInfo,
					Comment:    "创建触发器",
				})
			}
		case diff.DiffTypeRemoved:
			dropTriggerStatements = append(dropTriggerStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`;", td.TriggerName),
				ObjectType: "TRIGGER",
				ObjectName: td.TriggerName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除触发器",
			})
		case diff.DiffTypeModified:
			dropTriggerStatements = append(dropTriggerStatements, SQLStatement{
				SQL:        fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`;", td.TriggerName),
				ObjectType: "TRIGGER",
				ObjectName: td.TriggerName,
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    "删除触发器（将重建）",
			})
			if td.NewTrigger != nil {
				createTriggerStatements = append(createTriggerStatements, SQLStatement{
					SQL: fmt.Sprintf("CREATE TRIGGER `%s` %s %s ON `%s` FOR EACH ROW %s;",
						td.TriggerName, td.NewTrigger.Timing, td.NewTrigger.Event,
						td.NewTrigger.Table, td.NewTrigger.Statement),
					ObjectType: "TRIGGER",
					ObjectName: td.TriggerName,
					Operation:  "CREATE",
					Severity:   diff.SeverityWarning,
					Comment:    "重建触发器",
				})
			}
		}
	}

	// 按顺序合并所有语句
	script.Statements = append(script.Statements, dropFKStatements...)
	script.Statements = append(script.Statements, dropTriggerStatements...)
	script.Statements = append(script.Statements, dropViewStatements...)
	script.Statements = append(script.Statements, dropProcStatements...)
	script.Statements = append(script.Statements, dropFuncStatements...)
	script.Statements = append(script.Statements, dropTableStatements...)
	script.Statements = append(script.Statements, alterTableStatements...)
	script.Statements = append(script.Statements, createTableStatements...)
	script.Statements = append(script.Statements, createIndexStatements...)
	script.Statements = append(script.Statements, createFKStatements...)
	script.Statements = append(script.Statements, createFuncStatements...)
	script.Statements = append(script.Statements, createProcStatements...)
	script.Statements = append(script.Statements, createViewStatements...)
	script.Statements = append(script.Statements, createTriggerStatements...)

	// 生成完整SQL
	script.UpSQL = g.buildFullSQL(script.Statements, options)

	// 生成回滚脚本
	if options.IncludeRollback {
		script.DownSQL = g.buildRollbackSQL(script.Statements)
	}

	return script, nil
}

// generateColumnStatements 生成列变更语句
func (g *MySQLGenerator) generateColumnStatements(tableName string, cd *diff.ColumnDiff) []SQLStatement {
	var stmts []SQLStatement

	switch cd.DiffType {
	case diff.DiffTypeAdded:
		if cd.NewColumn != nil {
			sql := g.buildAddColumnSQL(tableName, cd.NewColumn)
			stmts = append(stmts, SQLStatement{
				SQL:        sql,
				ObjectType: "COLUMN",
				ObjectName: fmt.Sprintf("%s.%s", tableName, cd.ColumnName),
				Operation:  "ADD",
				Severity:   diff.SeverityInfo,
				Comment:    fmt.Sprintf("添加列 %s", cd.ColumnName),
			})
		}

	case diff.DiffTypeRemoved:
		stmts = append(stmts, SQLStatement{
			SQL:        fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", tableName, cd.ColumnName),
			ObjectType: "COLUMN",
			ObjectName: fmt.Sprintf("%s.%s", tableName, cd.ColumnName),
			Operation:  "DROP",
			Severity:   diff.SeverityDanger,
			Comment:    fmt.Sprintf("⚠️ 删除列 %s - 数据将丢失", cd.ColumnName),
		})

	case diff.DiffTypeModified:
		if cd.NewColumn != nil {
			sql := g.buildModifyColumnSQL(tableName, cd.NewColumn)
			stmts = append(stmts, SQLStatement{
				SQL:        sql,
				ObjectType: "COLUMN",
				ObjectName: fmt.Sprintf("%s.%s", tableName, cd.ColumnName),
				Operation:  "MODIFY",
				Severity:   cd.Severity,
				Comment:    fmt.Sprintf("修改列 %s", cd.ColumnName),
			})
		}
	}

	return stmts
}

// buildAddColumnSQL 构建添加列SQL
func (g *MySQLGenerator) buildAddColumnSQL(tableName string, col *extractor.ColumnSchema) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s`", tableName, col.Name))
	parts = append(parts, col.ColumnType)

	if !col.IsNullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "NULL")
	}

	if col.DefaultValue != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", g.formatDefaultValue(*col.DefaultValue, col.DataType)))
	}

	if col.IsAutoIncr {
		parts = append(parts, "AUTO_INCREMENT")
	}

	if col.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", g.escapeString(col.Comment)))
	}

	return strings.Join(parts, " ") + ";"
}

// buildModifyColumnSQL 构建修改列SQL
func (g *MySQLGenerator) buildModifyColumnSQL(tableName string, col *extractor.ColumnSchema) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s`", tableName, col.Name))
	parts = append(parts, col.ColumnType)

	if !col.IsNullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "NULL")
	}

	if col.DefaultValue != nil {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", g.formatDefaultValue(*col.DefaultValue, col.DataType)))
	}

	if col.IsAutoIncr {
		parts = append(parts, "AUTO_INCREMENT")
	}

	if col.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", g.escapeString(col.Comment)))
	}

	return strings.Join(parts, " ") + ";"
}

// generateIndexStatements 生成索引变更语句
func (g *MySQLGenerator) generateIndexStatements(tableName string, id *diff.IndexDiff) []SQLStatement {
	var stmts []SQLStatement

	switch id.DiffType {
	case diff.DiffTypeAdded:
		if id.NewIndex != nil {
			sql := g.buildCreateIndexSQL(tableName, id.NewIndex)
			stmts = append(stmts, SQLStatement{
				SQL:        sql,
				ObjectType: "INDEX",
				ObjectName: fmt.Sprintf("%s.%s", tableName, id.IndexName),
				Operation:  "CREATE",
				Severity:   diff.SeverityInfo,
				Comment:    fmt.Sprintf("创建索引 %s", id.IndexName),
			})
		}

	case diff.DiffTypeRemoved:
		if id.OldIndex != nil {
			var sql string
			if id.OldIndex.IsPrimary {
				sql = fmt.Sprintf("ALTER TABLE `%s` DROP PRIMARY KEY;", tableName)
			} else {
				sql = fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", tableName, id.IndexName)
			}
			stmts = append(stmts, SQLStatement{
				SQL:        sql,
				ObjectType: "INDEX",
				ObjectName: fmt.Sprintf("%s.%s", tableName, id.IndexName),
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    fmt.Sprintf("删除索引 %s", id.IndexName),
			})
		}

	case diff.DiffTypeModified:
		// 先删后加
		if id.OldIndex != nil {
			var dropSQL string
			if id.OldIndex.IsPrimary {
				dropSQL = fmt.Sprintf("ALTER TABLE `%s` DROP PRIMARY KEY;", tableName)
			} else {
				dropSQL = fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", tableName, id.IndexName)
			}
			stmts = append(stmts, SQLStatement{
				SQL:        dropSQL,
				ObjectType: "INDEX",
				ObjectName: fmt.Sprintf("%s.%s", tableName, id.IndexName),
				Operation:  "DROP",
				Severity:   diff.SeverityWarning,
				Comment:    fmt.Sprintf("删除索引 %s（将重建）", id.IndexName),
			})
		}
		if id.NewIndex != nil {
			sql := g.buildCreateIndexSQL(tableName, id.NewIndex)
			stmts = append(stmts, SQLStatement{
				SQL:        sql,
				ObjectType: "INDEX",
				ObjectName: fmt.Sprintf("%s.%s", tableName, id.IndexName),
				Operation:  "CREATE",
				Severity:   diff.SeverityWarning,
				Comment:    fmt.Sprintf("重建索引 %s", id.IndexName),
			})
		}
	}

	return stmts
}

// buildCreateIndexSQL 构建创建索引SQL
func (g *MySQLGenerator) buildCreateIndexSQL(tableName string, idx *extractor.IndexSchema) string {
	var columns []string
	for _, col := range idx.Columns {
		colDef := fmt.Sprintf("`%s`", col.Name)
		if col.SubPart != nil {
			colDef += fmt.Sprintf("(%d)", *col.SubPart)
		}
		columns = append(columns, colDef)
	}
	colList := strings.Join(columns, ", ")

	if idx.IsPrimary {
		return fmt.Sprintf("ALTER TABLE `%s` ADD PRIMARY KEY (%s);", tableName, colList)
	}

	var indexType string
	switch idx.Type {
	case extractor.IndexTypeUnique:
		indexType = "UNIQUE INDEX"
	case extractor.IndexTypeFulltext:
		indexType = "FULLTEXT INDEX"
	case extractor.IndexTypeSpatial:
		indexType = "SPATIAL INDEX"
	default:
		indexType = "INDEX"
	}

	return fmt.Sprintf("ALTER TABLE `%s` ADD %s `%s` (%s);", tableName, indexType, idx.Name, colList)
}

// generateAddForeignKey 生成添加外键语句
func (g *MySQLGenerator) generateAddForeignKey(tableName string, fk *extractor.ForeignKey) SQLStatement {
	columns := make([]string, len(fk.Columns))
	for i, col := range fk.Columns {
		columns[i] = fmt.Sprintf("`%s`", col)
	}

	refColumns := make([]string, len(fk.RefColumns))
	for i, col := range fk.RefColumns {
		refColumns[i] = fmt.Sprintf("`%s`", col)
	}

	sql := fmt.Sprintf("ALTER TABLE `%s` ADD CONSTRAINT `%s` FOREIGN KEY (%s) REFERENCES `%s` (%s)",
		tableName, fk.Name, strings.Join(columns, ", "),
		fk.RefTable, strings.Join(refColumns, ", "))

	if fk.OnDelete != "" && fk.OnDelete != "RESTRICT" {
		sql += " ON DELETE " + fk.OnDelete
	}
	if fk.OnUpdate != "" && fk.OnUpdate != "RESTRICT" {
		sql += " ON UPDATE " + fk.OnUpdate
	}
	sql += ";"

	return SQLStatement{
		SQL:        sql,
		ObjectType: "FOREIGN KEY",
		ObjectName: fmt.Sprintf("%s.%s", tableName, fk.Name),
		Operation:  "ADD",
		Severity:   diff.SeverityWarning,
		Comment:    fmt.Sprintf("添加外键 %s", fk.Name),
	}
}

// generateTablePropertyStatement 生成表属性变更语句
func (g *MySQLGenerator) generateTablePropertyStatement(tableName string, prop *diff.PropertyDiff) *SQLStatement {
	var sql string

	switch prop.Property {
	case "ENGINE":
		sql = fmt.Sprintf("ALTER TABLE `%s` ENGINE = %s;", tableName, prop.NewValue)
	case "CHARSET":
		sql = fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET %s;", tableName, prop.NewValue)
	case "COLLATION":
		// 字符集和排序规则一起处理
		return nil
	case "COMMENT":
		sql = fmt.Sprintf("ALTER TABLE `%s` COMMENT = '%s';", tableName, g.escapeString(prop.NewValue))
	default:
		return nil
	}

	return &SQLStatement{
		SQL:        sql,
		ObjectType: "TABLE",
		ObjectName: tableName,
		Operation:  "ALTER",
		Severity:   diff.SeverityInfo,
		Comment:    fmt.Sprintf("修改表属性 %s", prop.Property),
	}
}

// buildFullSQL 构建完整SQL
func (g *MySQLGenerator) buildFullSQL(statements []SQLStatement, options GenerateOptions) string {
	var builder strings.Builder

	// 添加头部注释
	if options.AddComments {
		builder.WriteString("-- ============================================\n")
		builder.WriteString("-- SchemaPatch 生成的数据库升级脚本\n")
		builder.WriteString(fmt.Sprintf("-- 生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
		builder.WriteString(fmt.Sprintf("-- 语句数量: %d\n", len(statements)))
		builder.WriteString("-- ============================================\n\n")
	}

	// 添加事务开始
	if options.WrapTransaction {
		builder.WriteString("START TRANSACTION;\n\n")
	}

	// 添加语句
	for i, stmt := range statements {
		if options.AddComments && stmt.Comment != "" {
			builder.WriteString(fmt.Sprintf("-- [%d/%d] %s\n", i+1, len(statements), stmt.Comment))
		}
		builder.WriteString(stmt.SQL)
		builder.WriteString("\n\n")
	}

	// 添加事务提交
	if options.WrapTransaction {
		builder.WriteString("COMMIT;\n")
	}

	return builder.String()
}

// buildRollbackSQL 构建回滚SQL
func (g *MySQLGenerator) buildRollbackSQL(statements []SQLStatement) string {
	var builder strings.Builder

	builder.WriteString("-- ============================================\n")
	builder.WriteString("-- 回滚脚本 (ROLLBACK)\n")
	builder.WriteString("-- 注意: 此脚本可能无法完全回滚所有更改\n")
	builder.WriteString("-- ============================================\n\n")

	// 逆序执行回滚
	for i := len(statements) - 1; i >= 0; i-- {
		stmt := statements[i]
		if stmt.RollbackSQL != "" {
			builder.WriteString(fmt.Sprintf("-- 回滚: %s\n", stmt.Comment))
			builder.WriteString(stmt.RollbackSQL)
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
}

// formatDefaultValue 格式化默认值
func (g *MySQLGenerator) formatDefaultValue(value, dataType string) string {
	// 特殊值处理
	upperValue := strings.ToUpper(value)
	if upperValue == "NULL" || upperValue == "CURRENT_TIMESTAMP" ||
		strings.HasPrefix(upperValue, "CURRENT_TIMESTAMP") {
		return value
	}

	// 数值类型不加引号
	lowerType := strings.ToLower(dataType)
	if strings.Contains(lowerType, "int") ||
		strings.Contains(lowerType, "decimal") ||
		strings.Contains(lowerType, "float") ||
		strings.Contains(lowerType, "double") {
		return value
	}

	// 字符串类型加引号
	return fmt.Sprintf("'%s'", g.escapeString(value))
}

// escapeString 转义字符串
func (g *MySQLGenerator) escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

