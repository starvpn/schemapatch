package extractor

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/starvpn/schemapatch/internal/config"
)

// MySQLExtractor MySQL Schema提取器
type MySQLExtractor struct {
	env *config.Environment
	db  *sql.DB
}

// NewMySQLExtractor 创建MySQL提取器
func NewMySQLExtractor(env *config.Environment) (*MySQLExtractor, error) {
	return &MySQLExtractor{env: env}, nil
}

// Connect 连接数据库
func (e *MySQLExtractor) Connect(ctx context.Context) error {
	dsn := e.buildDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 测试连接
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 确保字符集正确
	if _, err := db.ExecContext(ctx, "SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		db.Close()
		return fmt.Errorf("设置字符集失败: %w", err)
	}

	e.db = db
	return nil
}

// buildDSN 构建连接字符串
func (e *MySQLExtractor) buildDSN() string {
	charset := e.env.Charset
	if charset == "" {
		charset = "utf8mb4"
	}

	// 添加 collation 和 interpolateParams 确保中文正确处理
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&collation=utf8mb4_unicode_ci&parseTime=True&loc=Local&interpolateParams=true",
		e.env.Username,
		e.env.Password,
		e.env.Host,
		e.env.Port,
		e.env.Database,
		charset,
	)

	return dsn
}

// Close 关闭连接
func (e *MySQLExtractor) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

// TestConnection 测试连接
func (e *MySQLExtractor) TestConnection(ctx context.Context) error {
	if e.db == nil {
		if err := e.Connect(ctx); err != nil {
			return err
		}
		defer e.Close()
	}
	return e.db.PingContext(ctx)
}

// GetServerVersion 获取服务器版本
func (e *MySQLExtractor) GetServerVersion(ctx context.Context) (string, error) {
	var version string
	err := e.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	return version, err
}

// GetServerVariables 获取服务器变量
func (e *MySQLExtractor) GetServerVariables(ctx context.Context) (map[string]string, error) {
	rows, err := e.db.QueryContext(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	vars := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		vars[name] = value
	}
	return vars, nil
}

// ExtractSchema 提取完整Schema
func (e *MySQLExtractor) ExtractSchema(ctx context.Context, options ExtractOptions) (*DatabaseSchema, error) {
	schema := NewDatabaseSchema(e.env.Database)

	// 获取数据库字符集
	var dbCharset, dbCollation string
	err := e.db.QueryRowContext(ctx, `
		SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME 
		FROM information_schema.SCHEMATA 
		WHERE SCHEMA_NAME = ?
	`, e.env.Database).Scan(&dbCharset, &dbCollation)
	if err == nil {
		schema.Charset = dbCharset
		schema.Collation = dbCollation
	}

	// 提取表
	if options.IncludeTables {
		tables, err := e.ExtractTables(ctx)
		if err != nil {
			return nil, fmt.Errorf("提取表失败: %w", err)
		}
		schema.Tables = tables
	}

	// 提取视图
	if options.IncludeViews {
		views, err := e.ExtractViews(ctx)
		if err != nil {
			return nil, fmt.Errorf("提取视图失败: %w", err)
		}
		schema.Views = views
	}

	// 提取存储过程
	if options.IncludeProcedures {
		procedures, err := e.ExtractProcedures(ctx)
		if err != nil {
			return nil, fmt.Errorf("提取存储过程失败: %w", err)
		}
		schema.Procedures = procedures
	}

	// 提取函数
	if options.IncludeFunctions {
		functions, err := e.ExtractFunctions(ctx)
		if err != nil {
			return nil, fmt.Errorf("提取函数失败: %w", err)
		}
		schema.Functions = functions
	}

	// 提取触发器
	if options.IncludeTriggers {
		triggers, err := e.ExtractTriggers(ctx)
		if err != nil {
			return nil, fmt.Errorf("提取触发器失败: %w", err)
		}
		schema.Triggers = triggers
	}

	return schema, nil
}

// ExtractTables 提取表结构
func (e *MySQLExtractor) ExtractTables(ctx context.Context, tableNames ...string) (map[string]*TableSchema, error) {
	tables := make(map[string]*TableSchema)

	// 查询表信息
	query := `
		SELECT 
			TABLE_NAME, ENGINE, TABLE_COLLATION, TABLE_COMMENT, AUTO_INCREMENT
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
	`
	args := []interface{}{e.env.Database}

	if len(tableNames) > 0 {
		placeholders := make([]string, len(tableNames))
		for i, name := range tableNames {
			placeholders[i] = "?"
			args = append(args, name)
		}
		query += " AND TABLE_NAME IN (" + strings.Join(placeholders, ",") + ")"
	}

	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var table TableSchema
		var engine, collation, comment sql.NullString
		var autoIncr sql.NullInt64

		if err := rows.Scan(&table.Name, &engine, &collation, &comment, &autoIncr); err != nil {
			return nil, err
		}

		table.Engine = engine.String
		table.Collation = collation.String
		table.Comment = comment.String
		if autoIncr.Valid {
			table.AutoIncr = autoIncr.Int64
		}

		// 解析字符集
		if table.Collation != "" {
			parts := strings.Split(table.Collation, "_")
			if len(parts) > 0 {
				table.Charset = parts[0]
			}
		}

		tables[table.Name] = &table
	}

	// 为每个表提取列、索引、外键
	for tableName, table := range tables {
		// 提取列
		columns, err := e.extractColumns(ctx, tableName)
		if err != nil {
			return nil, fmt.Errorf("提取表 %s 的列失败: %w", tableName, err)
		}
		table.Columns = columns

		// 提取索引
		indexes, err := e.extractIndexes(ctx, tableName)
		if err != nil {
			return nil, fmt.Errorf("提取表 %s 的索引失败: %w", tableName, err)
		}
		table.Indexes = indexes

		// 提取外键
		foreignKeys, err := e.extractForeignKeys(ctx, tableName)
		if err != nil {
			return nil, fmt.Errorf("提取表 %s 的外键失败: %w", tableName, err)
		}
		table.ForeignKeys = foreignKeys

		// 获取CREATE TABLE语句
		createSQL, err := e.getCreateTableSQL(ctx, tableName)
		if err == nil {
			table.CreateSQL = createSQL
		}
	}

	return tables, nil
}

// extractColumns 提取表的列
func (e *MySQLExtractor) extractColumns(ctx context.Context, tableName string) ([]*ColumnSchema, error) {
	query := `
		SELECT 
			COLUMN_NAME, ORDINAL_POSITION, DATA_TYPE, COLUMN_TYPE,
			IS_NULLABLE, COLUMN_DEFAULT, EXTRA,
			CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE,
			CHARACTER_SET_NAME, COLLATION_NAME, COLUMN_COMMENT,
			GENERATION_EXPRESSION
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []*ColumnSchema
	for rows.Next() {
		var col ColumnSchema
		var isNullable string
		var defaultVal, extra, charsetName, collationName, comment, genExpr sql.NullString
		var charMaxLen, numPrec, numScale sql.NullInt64

		if err := rows.Scan(
			&col.Name, &col.Position, &col.DataType, &col.ColumnType,
			&isNullable, &defaultVal, &extra,
			&charMaxLen, &numPrec, &numScale,
			&charsetName, &collationName, &comment,
			&genExpr,
		); err != nil {
			return nil, err
		}

		col.IsNullable = isNullable == "YES"
		if defaultVal.Valid {
			col.DefaultValue = &defaultVal.String
		}
		col.Extra = extra.String
		col.IsAutoIncr = strings.Contains(strings.ToLower(extra.String), "auto_increment")
		if charMaxLen.Valid {
			col.CharMaxLen = &charMaxLen.Int64
		}
		if numPrec.Valid {
			col.NumericPrec = &numPrec.Int64
		}
		if numScale.Valid {
			col.NumericScale = &numScale.Int64
		}
		col.CharsetName = charsetName.String
		col.CollationName = collationName.String
		col.Comment = comment.String
		col.GeneratedExpr = genExpr.String
		col.IsGenerated = genExpr.Valid && genExpr.String != ""

		columns = append(columns, &col)
	}

	return columns, nil
}

// extractIndexes 提取表的索引
func (e *MySQLExtractor) extractIndexes(ctx context.Context, tableName string) (map[string]*IndexSchema, error) {
	query := `
		SELECT 
			INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX,
			SUB_PART, INDEX_TYPE, INDEX_COMMENT
		FROM information_schema.STATISTICS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*IndexSchema)
	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int
		var seqInIdx int
		var subPart sql.NullInt64
		var indexComment sql.NullString

		if err := rows.Scan(&indexName, &nonUnique, &columnName, &seqInIdx, &subPart, &indexType, &indexComment); err != nil {
			return nil, err
		}

		idx, exists := indexes[indexName]
		if !exists {
			idx = &IndexSchema{
				Name:      indexName,
				IsUnique:  nonUnique == 0,
				IsPrimary: indexName == "PRIMARY",
				IndexType: indexType,
				Comment:   indexComment.String,
				Columns:   []IndexColumn{},
			}

			// 设置索引类型
			if idx.IsPrimary {
				idx.Type = IndexTypePrimary
			} else if idx.IsUnique {
				idx.Type = IndexTypeUnique
			} else if indexType == "FULLTEXT" {
				idx.Type = IndexTypeFulltext
			} else if indexType == "SPATIAL" {
				idx.Type = IndexTypeSpatial
			} else {
				idx.Type = IndexTypeNormal
			}

			indexes[indexName] = idx
		}

		idxCol := IndexColumn{
			Name:     columnName,
			SeqInIdx: seqInIdx,
		}
		if subPart.Valid {
			sp := int(subPart.Int64)
			idxCol.SubPart = &sp
		}
		idx.Columns = append(idx.Columns, idxCol)
	}

	return indexes, nil
}

// extractForeignKeys 提取表的外键
func (e *MySQLExtractor) extractForeignKeys(ctx context.Context, tableName string) (map[string]*ForeignKey, error) {
	query := `
		SELECT 
			kcu.CONSTRAINT_NAME,
			kcu.COLUMN_NAME,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_COLUMN_NAME,
			rc.DELETE_RULE,
			rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
			AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ? 
			AND kcu.TABLE_NAME = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	foreignKeys := make(map[string]*ForeignKey)
	for rows.Next() {
		var fkName, columnName, refTable, refColumn, onDelete, onUpdate string

		if err := rows.Scan(&fkName, &columnName, &refTable, &refColumn, &onDelete, &onUpdate); err != nil {
			return nil, err
		}

		fk, exists := foreignKeys[fkName]
		if !exists {
			fk = &ForeignKey{
				Name:       fkName,
				Columns:    []string{},
				RefTable:   refTable,
				RefColumns: []string{},
				OnDelete:   onDelete,
				OnUpdate:   onUpdate,
			}
			foreignKeys[fkName] = fk
		}

		fk.Columns = append(fk.Columns, columnName)
		fk.RefColumns = append(fk.RefColumns, refColumn)
	}

	return foreignKeys, nil
}

// getCreateTableSQL 获取CREATE TABLE语句
func (e *MySQLExtractor) getCreateTableSQL(ctx context.Context, tableName string) (string, error) {
	var name, createSQL string
	err := e.db.QueryRowContext(ctx, "SHOW CREATE TABLE `"+tableName+"`").Scan(&name, &createSQL)
	return createSQL, err
}

// ExtractViews 提取视图
func (e *MySQLExtractor) ExtractViews(ctx context.Context) (map[string]*ViewSchema, error) {
	query := `
		SELECT 
			TABLE_NAME, VIEW_DEFINITION, DEFINER, SECURITY_TYPE, CHECK_OPTION
		FROM information_schema.VIEWS 
		WHERE TABLE_SCHEMA = ?
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := make(map[string]*ViewSchema)
	for rows.Next() {
		var view ViewSchema
		var definition, definer, security, checkOpt sql.NullString

		if err := rows.Scan(&view.Name, &definition, &definer, &security, &checkOpt); err != nil {
			return nil, err
		}

		view.Definition = definition.String
		view.Definer = definer.String
		view.Security = security.String
		view.CheckOpt = checkOpt.String

		views[view.Name] = &view
	}

	return views, nil
}

// ExtractProcedures 提取存储过程
func (e *MySQLExtractor) ExtractProcedures(ctx context.Context) (map[string]*ProcedureSchema, error) {
	// 先获取存储过程列表和元数据
	query := `
		SELECT 
			ROUTINE_NAME, DEFINER, SECURITY_TYPE, SQL_MODE, ROUTINE_COMMENT
		FROM information_schema.ROUTINES 
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_TYPE = 'PROCEDURE'
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	procedures := make(map[string]*ProcedureSchema)
	for rows.Next() {
		var proc ProcedureSchema
		var definer, security, sqlMode, comment sql.NullString

		if err := rows.Scan(&proc.Name, &definer, &security, &sqlMode, &comment); err != nil {
			return nil, err
		}

		proc.Definer = definer.String
		proc.Security = security.String
		proc.SQLMode = sqlMode.String
		proc.Comment = comment.String

		procedures[proc.Name] = &proc
	}

	// 使用 SHOW CREATE PROCEDURE 获取完整定义
	for name, proc := range procedures {
		createSQL, err := e.getRoutineCreateSQL(ctx, "PROCEDURE", name)
		if err != nil {
			// 记录警告但继续
			proc.Definition = ""
		} else {
			proc.Definition = createSQL
		}

		// 提取参数
		params, err := e.extractRoutineParams(ctx, proc.Name, "PROCEDURE")
		if err == nil {
			proc.Params = params
		}
	}

	return procedures, nil
}

// ExtractFunctions 提取函数
func (e *MySQLExtractor) ExtractFunctions(ctx context.Context) (map[string]*FunctionSchema, error) {
	// 先获取函数列表和元数据
	query := `
		SELECT 
			ROUTINE_NAME, DEFINER, SECURITY_TYPE, SQL_MODE, ROUTINE_COMMENT,
			DTD_IDENTIFIER, IS_DETERMINISTIC
		FROM information_schema.ROUTINES 
		WHERE ROUTINE_SCHEMA = ? AND ROUTINE_TYPE = 'FUNCTION'
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	functions := make(map[string]*FunctionSchema)
	for rows.Next() {
		var fn FunctionSchema
		var definer, security, sqlMode, comment, returns, isDetermin sql.NullString

		if err := rows.Scan(&fn.Name, &definer, &security, &sqlMode, &comment, &returns, &isDetermin); err != nil {
			return nil, err
		}

		fn.Definer = definer.String
		fn.Security = security.String
		fn.SQLMode = sqlMode.String
		fn.Comment = comment.String
		fn.Returns = returns.String
		fn.IsDetermin = isDetermin.String == "YES"

		functions[fn.Name] = &fn
	}

	// 使用 SHOW CREATE FUNCTION 获取完整定义
	for name, fn := range functions {
		createSQL, err := e.getRoutineCreateSQL(ctx, "FUNCTION", name)
		if err != nil {
			fn.Definition = ""
		} else {
			fn.Definition = createSQL
		}

		// 提取参数
		params, err := e.extractRoutineParams(ctx, fn.Name, "FUNCTION")
		if err == nil {
			fn.Params = params
		}
	}

	return functions, nil
}

// getRoutineCreateSQL 使用 SHOW CREATE 获取完整的存储过程/函数定义
func (e *MySQLExtractor) getRoutineCreateSQL(ctx context.Context, routineType, name string) (string, error) {
	var query string
	if routineType == "PROCEDURE" {
		query = fmt.Sprintf("SHOW CREATE PROCEDURE `%s`", name)
	} else {
		query = fmt.Sprintf("SHOW CREATE FUNCTION `%s`", name)
	}

	row := e.db.QueryRowContext(ctx, query)

	var routineName, sqlMode, createSQL, charset, collation, dbCollation sql.NullString
	err := row.Scan(&routineName, &sqlMode, &createSQL, &charset, &collation, &dbCollation)
	if err != nil {
		return "", err
	}

	return createSQL.String, nil
}

// extractRoutineParams 提取存储过程/函数参数
func (e *MySQLExtractor) extractRoutineParams(ctx context.Context, routineName, routineType string) ([]ProcedureParam, error) {
	query := `
		SELECT 
			PARAMETER_NAME, PARAMETER_MODE, DTD_IDENTIFIER, ORDINAL_POSITION
		FROM information_schema.PARAMETERS 
		WHERE SPECIFIC_SCHEMA = ? AND SPECIFIC_NAME = ? AND ROUTINE_TYPE = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database, routineName, routineType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var params []ProcedureParam
	for rows.Next() {
		var param ProcedureParam
		var name, mode, dataType sql.NullString

		if err := rows.Scan(&name, &mode, &dataType, &param.Position); err != nil {
			return nil, err
		}

		param.Name = name.String
		param.Mode = mode.String
		param.DataType = dataType.String

		// 跳过返回值（ORDINAL_POSITION = 0）
		if param.Position > 0 {
			params = append(params, param)
		}
	}

	return params, nil
}

// ExtractTriggers 提取触发器
func (e *MySQLExtractor) ExtractTriggers(ctx context.Context) (map[string]*TriggerSchema, error) {
	query := `
		SELECT 
			TRIGGER_NAME, EVENT_OBJECT_TABLE, EVENT_MANIPULATION,
			ACTION_TIMING, ACTION_STATEMENT, DEFINER, SQL_MODE
		FROM information_schema.TRIGGERS 
		WHERE TRIGGER_SCHEMA = ?
	`

	rows, err := e.db.QueryContext(ctx, query, e.env.Database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	triggers := make(map[string]*TriggerSchema)
	for rows.Next() {
		var trigger TriggerSchema
		var definer, sqlMode sql.NullString

		if err := rows.Scan(&trigger.Name, &trigger.Table, &trigger.Event, &trigger.Timing, &trigger.Statement, &definer, &sqlMode); err != nil {
			return nil, err
		}

		trigger.Definer = definer.String
		trigger.SQLMode = sqlMode.String

		triggers[trigger.Name] = &trigger
	}

	return triggers, nil
}
