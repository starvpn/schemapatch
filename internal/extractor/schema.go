package extractor

import (
	"time"
)

// DatabaseSchema 数据库完整Schema
type DatabaseSchema struct {
	Database    string                      `json:"database"`
	Charset     string                      `json:"charset"`
	Collation   string                      `json:"collation"`
	Tables      map[string]*TableSchema     `json:"tables"`
	Views       map[string]*ViewSchema      `json:"views"`
	Procedures  map[string]*ProcedureSchema `json:"procedures"`
	Functions   map[string]*FunctionSchema  `json:"functions"`
	Triggers    map[string]*TriggerSchema   `json:"triggers"`
	ExtractedAt time.Time                   `json:"extracted_at"`
}

// TableSchema 表结构
type TableSchema struct {
	Name        string                   `json:"name"`
	Engine      string                   `json:"engine"`
	Charset     string                   `json:"charset"`
	Collation   string                   `json:"collation"`
	Comment     string                   `json:"comment"`
	AutoIncr    int64                    `json:"auto_incr"`
	Columns     []*ColumnSchema          `json:"columns"`
	Indexes     map[string]*IndexSchema  `json:"indexes"`
	ForeignKeys map[string]*ForeignKey   `json:"foreign_keys"`
	CreateSQL   string                   `json:"create_sql"`
}

// ColumnSchema 列结构
type ColumnSchema struct {
	Name          string  `json:"name"`
	Position      int     `json:"position"`
	DataType      string  `json:"data_type"`       // 如 varchar, int, bigint
	ColumnType    string  `json:"column_type"`     // 完整类型如 varchar(255), int(11) unsigned
	IsNullable    bool    `json:"is_nullable"`
	DefaultValue  *string `json:"default_value"`   // nil表示无默认值
	IsAutoIncr    bool    `json:"is_auto_incr"`
	CharMaxLen    *int64  `json:"char_max_len"`    // 字符最大长度
	NumericPrec   *int64  `json:"numeric_prec"`    // 数值精度
	NumericScale  *int64  `json:"numeric_scale"`   // 数值小数位
	CharsetName   string  `json:"charset_name"`
	CollationName string  `json:"collation_name"`
	Comment       string  `json:"comment"`
	Extra         string  `json:"extra"`           // 其他属性如 on update CURRENT_TIMESTAMP
	GeneratedExpr string  `json:"generated_expr"`  // 生成列表达式
	IsGenerated   bool    `json:"is_generated"`    // 是否是生成列
}

// IndexSchema 索引结构
type IndexSchema struct {
	Name       string        `json:"name"`
	Type       IndexType     `json:"type"`
	IsUnique   bool          `json:"is_unique"`
	IsPrimary  bool          `json:"is_primary"`
	Columns    []IndexColumn `json:"columns"`
	Comment    string        `json:"comment"`
	IndexType  string        `json:"index_type"` // BTREE, HASH, FULLTEXT
}

// IndexColumn 索引列
type IndexColumn struct {
	Name      string `json:"name"`
	SeqInIdx  int    `json:"seq_in_idx"`
	SubPart   *int   `json:"sub_part"`   // 前缀索引长度
	IsDesc    bool   `json:"is_desc"`    // 是否降序 (MySQL 8.0+)
}

// IndexType 索引类型
type IndexType int

const (
	IndexTypePrimary IndexType = iota
	IndexTypeUnique
	IndexTypeNormal
	IndexTypeFulltext
	IndexTypeSpatial
)

func (t IndexType) String() string {
	switch t {
	case IndexTypePrimary:
		return "PRIMARY"
	case IndexTypeUnique:
		return "UNIQUE"
	case IndexTypeNormal:
		return "INDEX"
	case IndexTypeFulltext:
		return "FULLTEXT"
	case IndexTypeSpatial:
		return "SPATIAL"
	default:
		return "UNKNOWN"
	}
}

// ForeignKey 外键结构
type ForeignKey struct {
	Name             string   `json:"name"`
	Columns          []string `json:"columns"`
	RefTable         string   `json:"ref_table"`
	RefColumns       []string `json:"ref_columns"`
	OnDelete         string   `json:"on_delete"` // CASCADE, SET NULL, RESTRICT, NO ACTION
	OnUpdate         string   `json:"on_update"`
}

// ViewSchema 视图结构
type ViewSchema struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	Definer    string `json:"definer"`
	Security   string `json:"security"` // DEFINER or INVOKER
	CheckOpt   string `json:"check_opt"`
}

// ProcedureSchema 存储过程结构
type ProcedureSchema struct {
	Name       string            `json:"name"`
	Definition string            `json:"definition"`
	Definer    string            `json:"definer"`
	Params     []ProcedureParam  `json:"params"`
	Comment    string            `json:"comment"`
	Security   string            `json:"security"`
	SQLMode    string            `json:"sql_mode"`
}

// ProcedureParam 存储过程参数
type ProcedureParam struct {
	Name     string `json:"name"`
	Mode     string `json:"mode"` // IN, OUT, INOUT
	DataType string `json:"data_type"`
	Position int    `json:"position"`
}

// FunctionSchema 函数结构
type FunctionSchema struct {
	Name       string           `json:"name"`
	Definition string           `json:"definition"`
	Definer    string           `json:"definer"`
	Params     []ProcedureParam `json:"params"`
	Returns    string           `json:"returns"`
	Comment    string           `json:"comment"`
	Security   string           `json:"security"`
	SQLMode    string           `json:"sql_mode"`
	IsDetermin bool             `json:"is_deterministic"`
}

// TriggerSchema 触发器结构
type TriggerSchema struct {
	Name       string `json:"name"`
	Table      string `json:"table"`
	Event      string `json:"event"`   // INSERT, UPDATE, DELETE
	Timing     string `json:"timing"`  // BEFORE, AFTER
	Statement  string `json:"statement"`
	Definer    string `json:"definer"`
	SQLMode    string `json:"sql_mode"`
}

// NewDatabaseSchema 创建空的数据库Schema
func NewDatabaseSchema(database string) *DatabaseSchema {
	return &DatabaseSchema{
		Database:    database,
		Tables:      make(map[string]*TableSchema),
		Views:       make(map[string]*ViewSchema),
		Procedures:  make(map[string]*ProcedureSchema),
		Functions:   make(map[string]*FunctionSchema),
		Triggers:    make(map[string]*TriggerSchema),
		ExtractedAt: time.Now(),
	}
}

// GetColumn 根据名称获取列
func (t *TableSchema) GetColumn(name string) *ColumnSchema {
	for _, col := range t.Columns {
		if col.Name == name {
			return col
		}
	}
	return nil
}

// GetColumnNames 获取所有列名
func (t *TableSchema) GetColumnNames() []string {
	names := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = col.Name
	}
	return names
}

// GetPrimaryKey 获取主键索引
func (t *TableSchema) GetPrimaryKey() *IndexSchema {
	for _, idx := range t.Indexes {
		if idx.IsPrimary {
			return idx
		}
	}
	return nil
}

// Clone 深拷贝Schema
func (s *DatabaseSchema) Clone() *DatabaseSchema {
	clone := NewDatabaseSchema(s.Database)
	clone.Charset = s.Charset
	clone.Collation = s.Collation
	clone.ExtractedAt = s.ExtractedAt

	// 复制表
	for name, table := range s.Tables {
		clone.Tables[name] = table // TODO: 深拷贝
	}

	// 复制视图
	for name, view := range s.Views {
		clone.Views[name] = view
	}

	// 复制存储过程
	for name, proc := range s.Procedures {
		clone.Procedures[name] = proc
	}

	// 复制函数
	for name, fn := range s.Functions {
		clone.Functions[name] = fn
	}

	// 复制触发器
	for name, trigger := range s.Triggers {
		clone.Triggers[name] = trigger
	}

	return clone
}

// Statistics 返回Schema统计信息
func (s *DatabaseSchema) Statistics() map[string]int {
	return map[string]int{
		"tables":     len(s.Tables),
		"views":      len(s.Views),
		"procedures": len(s.Procedures),
		"functions":  len(s.Functions),
		"triggers":   len(s.Triggers),
	}
}
