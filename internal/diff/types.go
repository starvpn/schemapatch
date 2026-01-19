package diff

import (
	"time"

	"github.com/starvpn/schemapatch/internal/extractor"
)

// DiffType 差异类型
type DiffType int

const (
	DiffTypeAdded    DiffType = iota // 新增
	DiffTypeRemoved                  // 删除
	DiffTypeModified                 // 修改
)

func (t DiffType) String() string {
	switch t {
	case DiffTypeAdded:
		return "新增"
	case DiffTypeRemoved:
		return "删除"
	case DiffTypeModified:
		return "修改"
	default:
		return "未知"
	}
}

// DiffSeverity 差异严重程度
type DiffSeverity int

const (
	SeverityInfo    DiffSeverity = iota // 信息 (如注释变更)
	SeverityWarning                     // 警告 (如类型扩展)
	SeverityDanger                      // 危险 (如删除列/表、类型收缩)
)

func (s DiffSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "信息"
	case SeverityWarning:
		return "警告"
	case SeverityDanger:
		return "危险"
	default:
		return "未知"
	}
}

// SchemaDiff 完整的Schema差异
type SchemaDiff struct {
	SourceEnv    string           `json:"source_env"`
	TargetEnv    string           `json:"target_env"`
	TableDiffs   []TableDiff      `json:"table_diffs"`
	ViewDiffs    []ViewDiff       `json:"view_diffs"`
	ProcDiffs    []ProcedureDiff  `json:"proc_diffs"`
	FuncDiffs    []FunctionDiff   `json:"func_diffs"`
	TriggerDiffs []TriggerDiff    `json:"trigger_diffs"`
	Statistics   DiffStatistics   `json:"statistics"`
	GeneratedAt  time.Time        `json:"generated_at"`
}

// DiffStatistics 差异统计
type DiffStatistics struct {
	TotalDiffs    int `json:"total_diffs"`
	TablesAdded   int `json:"tables_added"`
	TablesRemoved int `json:"tables_removed"`
	TablesChanged int `json:"tables_changed"`
	ViewsAdded    int `json:"views_added"`
	ViewsRemoved  int `json:"views_removed"`
	ViewsChanged  int `json:"views_changed"`
	ProcsAdded    int `json:"procs_added"`
	ProcsRemoved  int `json:"procs_removed"`
	ProcsChanged  int `json:"procs_changed"`
	FuncsAdded    int `json:"funcs_added"`
	FuncsRemoved  int `json:"funcs_removed"`
	FuncsChanged  int `json:"funcs_changed"`
	TriggersAdded   int `json:"triggers_added"`
	TriggersRemoved int `json:"triggers_removed"`
	TriggersChanged int `json:"triggers_changed"`
	DangerCount   int `json:"danger_count"`
	WarningCount  int `json:"warning_count"`
	InfoCount     int `json:"info_count"`
}

// TableDiff 表差异
type TableDiff struct {
	TableName   string                  `json:"table_name"`
	DiffType    DiffType                `json:"diff_type"`
	Severity    DiffSeverity            `json:"severity"`
	OldTable    *extractor.TableSchema  `json:"old_table,omitempty"`
	NewTable    *extractor.TableSchema  `json:"new_table,omitempty"`
	ColumnDiffs []ColumnDiff            `json:"column_diffs,omitempty"`
	IndexDiffs  []IndexDiff             `json:"index_diffs,omitempty"`
	FKeyDiffs   []ForeignKeyDiff        `json:"fkey_diffs,omitempty"`
	TableProps  []PropertyDiff          `json:"table_props,omitempty"` // 表属性变更(引擎、字符集等)
	Description string                  `json:"description"`
}

// ColumnDiff 列差异
type ColumnDiff struct {
	ColumnName    string                   `json:"column_name"`
	DiffType      DiffType                 `json:"diff_type"`
	Severity      DiffSeverity             `json:"severity"`
	OldColumn     *extractor.ColumnSchema  `json:"old_column,omitempty"`
	NewColumn     *extractor.ColumnSchema  `json:"new_column,omitempty"`
	Changes       []PropertyDiff           `json:"changes,omitempty"`
	RiskNote      string                   `json:"risk_note"`
}

// IndexDiff 索引差异
type IndexDiff struct {
	IndexName   string                  `json:"index_name"`
	DiffType    DiffType                `json:"diff_type"`
	Severity    DiffSeverity            `json:"severity"`
	OldIndex    *extractor.IndexSchema  `json:"old_index,omitempty"`
	NewIndex    *extractor.IndexSchema  `json:"new_index,omitempty"`
	Changes     []PropertyDiff          `json:"changes,omitempty"`
	Description string                  `json:"description"`
}

// ForeignKeyDiff 外键差异
type ForeignKeyDiff struct {
	FKeyName    string                `json:"fkey_name"`
	DiffType    DiffType              `json:"diff_type"`
	Severity    DiffSeverity          `json:"severity"`
	OldFKey     *extractor.ForeignKey `json:"old_fkey,omitempty"`
	NewFKey     *extractor.ForeignKey `json:"new_fkey,omitempty"`
	Description string                `json:"description"`
}

// ViewDiff 视图差异
type ViewDiff struct {
	ViewName    string                 `json:"view_name"`
	DiffType    DiffType               `json:"diff_type"`
	Severity    DiffSeverity           `json:"severity"`
	OldView     *extractor.ViewSchema  `json:"old_view,omitempty"`
	NewView     *extractor.ViewSchema  `json:"new_view,omitempty"`
	Description string                 `json:"description"`
}

// ProcedureDiff 存储过程差异
type ProcedureDiff struct {
	ProcName    string                      `json:"proc_name"`
	DiffType    DiffType                    `json:"diff_type"`
	Severity    DiffSeverity                `json:"severity"`
	OldProc     *extractor.ProcedureSchema  `json:"old_proc,omitempty"`
	NewProc     *extractor.ProcedureSchema  `json:"new_proc,omitempty"`
	Description string                      `json:"description"`
}

// FunctionDiff 函数差异
type FunctionDiff struct {
	FuncName    string                     `json:"func_name"`
	DiffType    DiffType                   `json:"diff_type"`
	Severity    DiffSeverity               `json:"severity"`
	OldFunc     *extractor.FunctionSchema  `json:"old_func,omitempty"`
	NewFunc     *extractor.FunctionSchema  `json:"new_func,omitempty"`
	Description string                     `json:"description"`
}

// TriggerDiff 触发器差异
type TriggerDiff struct {
	TriggerName string                    `json:"trigger_name"`
	DiffType    DiffType                  `json:"diff_type"`
	Severity    DiffSeverity              `json:"severity"`
	OldTrigger  *extractor.TriggerSchema  `json:"old_trigger,omitempty"`
	NewTrigger  *extractor.TriggerSchema  `json:"new_trigger,omitempty"`
	Description string                    `json:"description"`
}

// PropertyDiff 属性差异
type PropertyDiff struct {
	Property string `json:"property"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// HasDiff 检查是否有差异
func (d *SchemaDiff) HasDiff() bool {
	return len(d.TableDiffs) > 0 ||
		len(d.ViewDiffs) > 0 ||
		len(d.ProcDiffs) > 0 ||
		len(d.FuncDiffs) > 0 ||
		len(d.TriggerDiffs) > 0
}

// GetMaxSeverity 获取最高严重程度
func (d *SchemaDiff) GetMaxSeverity() DiffSeverity {
	max := SeverityInfo

	for _, td := range d.TableDiffs {
		if td.Severity > max {
			max = td.Severity
		}
		for _, cd := range td.ColumnDiffs {
			if cd.Severity > max {
				max = cd.Severity
			}
		}
	}

	for _, vd := range d.ViewDiffs {
		if vd.Severity > max {
			max = vd.Severity
		}
	}

	for _, pd := range d.ProcDiffs {
		if pd.Severity > max {
			max = pd.Severity
		}
	}

	for _, fd := range d.FuncDiffs {
		if fd.Severity > max {
			max = fd.Severity
		}
	}

	for _, td := range d.TriggerDiffs {
		if td.Severity > max {
			max = td.Severity
		}
	}

	return max
}

// CountBySeverity 按严重程度统计
func (d *SchemaDiff) CountBySeverity() map[DiffSeverity]int {
	counts := make(map[DiffSeverity]int)

	for _, td := range d.TableDiffs {
		counts[td.Severity]++
		for _, cd := range td.ColumnDiffs {
			counts[cd.Severity]++
		}
		for _, id := range td.IndexDiffs {
			counts[id.Severity]++
		}
		for _, fkd := range td.FKeyDiffs {
			counts[fkd.Severity]++
		}
	}

	for _, vd := range d.ViewDiffs {
		counts[vd.Severity]++
	}

	for _, pd := range d.ProcDiffs {
		counts[pd.Severity]++
	}

	for _, fd := range d.FuncDiffs {
		counts[fd.Severity]++
	}

	for _, td := range d.TriggerDiffs {
		counts[td.Severity]++
	}

	return counts
}
