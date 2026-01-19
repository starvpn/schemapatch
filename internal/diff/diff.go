package diff

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/starvpn/schemapatch/internal/config"
	"github.com/starvpn/schemapatch/internal/extractor"
)

// DiffEngine 差异分析引擎
type DiffEngine struct {
	ignoreRules config.IgnoreConfig
}

// NewDiffEngine 创建差异分析引擎
func NewDiffEngine(ignoreRules config.IgnoreConfig) *DiffEngine {
	return &DiffEngine{ignoreRules: ignoreRules}
}

// Compare 比较两个Schema
// source: 开发环境Schema (新的)
// target: 生产环境Schema (旧的)
// 返回: 需要从target升级到source的差异
func (e *DiffEngine) Compare(source, target *extractor.DatabaseSchema) *SchemaDiff {
	diff := &SchemaDiff{
		SourceEnv:   source.Database,
		TargetEnv:   target.Database,
		GeneratedAt: time.Now(),
	}

	// 比较表
	diff.TableDiffs = e.compareTables(source.Tables, target.Tables)

	// 比较视图
	diff.ViewDiffs = e.compareViews(source.Views, target.Views)

	// 比较存储过程
	diff.ProcDiffs = e.compareProcedures(source.Procedures, target.Procedures)

	// 比较函数
	diff.FuncDiffs = e.compareFunctions(source.Functions, target.Functions)

	// 比较触发器
	diff.TriggerDiffs = e.compareTriggers(source.Triggers, target.Triggers)

	// 计算统计信息
	diff.Statistics = e.calculateStatistics(diff)

	return diff
}

// compareTables 比较表
func (e *DiffEngine) compareTables(sourceTables, targetTables map[string]*extractor.TableSchema) []TableDiff {
	var diffs []TableDiff

	// 构建比较选项
	opts := TableCompareOptions{
		IgnoreComments:  e.ignoreRules.IgnoreComments,
		IgnoreCharset:   e.ignoreRules.IgnoreCharset,
		IgnoreCollation: e.ignoreRules.IgnoreCollation,
	}

	// 检查新增和修改的表
	for name, srcTable := range sourceTables {
		// 检查是否忽略
		if e.shouldIgnoreTable(name) {
			continue
		}

		tgtTable, exists := targetTables[name]
		if !exists {
			// 新增表
			diffs = append(diffs, TableDiff{
				TableName:   name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewTable:    srcTable,
				Description: "新增表",
			})
		} else {
			// 比较表结构（使用选项）
			tableDiff := compareTablesWithOptions(srcTable, tgtTable, opts)
			
			// 过滤忽略的列
			tableDiff.ColumnDiffs = e.filterIgnoredColumns(name, tableDiff.ColumnDiffs)
			
			// 如果有差异，添加到结果
			if len(tableDiff.ColumnDiffs) > 0 || 
			   len(tableDiff.IndexDiffs) > 0 || 
			   len(tableDiff.FKeyDiffs) > 0 ||
			   len(tableDiff.TableProps) > 0 {
				diffs = append(diffs, *tableDiff)
			}
		}
	}

	// 检查删除的表
	for name, tgtTable := range targetTables {
		if e.shouldIgnoreTable(name) {
			continue
		}

		if _, exists := sourceTables[name]; !exists {
			diffs = append(diffs, TableDiff{
				TableName:   name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityDanger,
				OldTable:    tgtTable,
				Description: "删除表 - 数据将丢失",
			})
		}
	}

	return diffs
}

// compareViews 比较视图
func (e *DiffEngine) compareViews(sourceViews, targetViews map[string]*extractor.ViewSchema) []ViewDiff {
	var diffs []ViewDiff

	// 检查新增和修改的视图
	for name, srcView := range sourceViews {
		tgtView, exists := targetViews[name]
		if !exists {
			diffs = append(diffs, ViewDiff{
				ViewName:    name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewView:     srcView,
				Description: "新增视图",
			})
		} else {
			// 比较视图定义
			if srcView.Definition != tgtView.Definition {
				diffs = append(diffs, ViewDiff{
					ViewName:    name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldView:     tgtView,
					NewView:     srcView,
					Description: "视图定义已变更",
				})
			}
		}
	}

	// 检查删除的视图
	for name, tgtView := range targetViews {
		if _, exists := sourceViews[name]; !exists {
			diffs = append(diffs, ViewDiff{
				ViewName:    name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldView:     tgtView,
				Description: "删除视图",
			})
		}
	}

	return diffs
}

// compareProcedures 比较存储过程
func (e *DiffEngine) compareProcedures(sourceProcs, targetProcs map[string]*extractor.ProcedureSchema) []ProcedureDiff {
	var diffs []ProcedureDiff

	for name, srcProc := range sourceProcs {
		tgtProc, exists := targetProcs[name]
		if !exists {
			diffs = append(diffs, ProcedureDiff{
				ProcName:    name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewProc:     srcProc,
				Description: "新增存储过程",
			})
		} else {
			if srcProc.Definition != tgtProc.Definition {
				diffs = append(diffs, ProcedureDiff{
					ProcName:    name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldProc:     tgtProc,
					NewProc:     srcProc,
					Description: "存储过程定义已变更",
				})
			}
		}
	}

	for name, tgtProc := range targetProcs {
		if _, exists := sourceProcs[name]; !exists {
			diffs = append(diffs, ProcedureDiff{
				ProcName:    name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldProc:     tgtProc,
				Description: "删除存储过程",
			})
		}
	}

	return diffs
}

// compareFunctions 比较函数
func (e *DiffEngine) compareFunctions(sourceFuncs, targetFuncs map[string]*extractor.FunctionSchema) []FunctionDiff {
	var diffs []FunctionDiff

	for name, srcFunc := range sourceFuncs {
		tgtFunc, exists := targetFuncs[name]
		if !exists {
			diffs = append(diffs, FunctionDiff{
				FuncName:    name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewFunc:     srcFunc,
				Description: "新增函数",
			})
		} else {
			if srcFunc.Definition != tgtFunc.Definition || srcFunc.Returns != tgtFunc.Returns {
				diffs = append(diffs, FunctionDiff{
					FuncName:    name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldFunc:     tgtFunc,
					NewFunc:     srcFunc,
					Description: "函数定义已变更",
				})
			}
		}
	}

	for name, tgtFunc := range targetFuncs {
		if _, exists := sourceFuncs[name]; !exists {
			diffs = append(diffs, FunctionDiff{
				FuncName:    name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldFunc:     tgtFunc,
				Description: "删除函数",
			})
		}
	}

	return diffs
}

// compareTriggers 比较触发器
func (e *DiffEngine) compareTriggers(sourceTriggers, targetTriggers map[string]*extractor.TriggerSchema) []TriggerDiff {
	var diffs []TriggerDiff

	for name, srcTrigger := range sourceTriggers {
		tgtTrigger, exists := targetTriggers[name]
		if !exists {
			diffs = append(diffs, TriggerDiff{
				TriggerName: name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewTrigger:  srcTrigger,
				Description: "新增触发器",
			})
		} else {
			if srcTrigger.Statement != tgtTrigger.Statement ||
				srcTrigger.Event != tgtTrigger.Event ||
				srcTrigger.Timing != tgtTrigger.Timing {
				diffs = append(diffs, TriggerDiff{
					TriggerName: name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldTrigger:  tgtTrigger,
					NewTrigger:  srcTrigger,
					Description: "触发器定义已变更",
				})
			}
		}
	}

	for name, tgtTrigger := range targetTriggers {
		if _, exists := sourceTriggers[name]; !exists {
			diffs = append(diffs, TriggerDiff{
				TriggerName: name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldTrigger:  tgtTrigger,
				Description: "删除触发器",
			})
		}
	}

	return diffs
}

// shouldIgnoreTable 检查是否应该忽略表
func (e *DiffEngine) shouldIgnoreTable(tableName string) bool {
	for _, pattern := range e.ignoreRules.Tables {
		if matched, _ := filepath.Match(pattern, tableName); matched {
			return true
		}
	}
	return false
}

// filterIgnoredColumns 过滤忽略的列
func (e *DiffEngine) filterIgnoredColumns(tableName string, diffs []ColumnDiff) []ColumnDiff {
	var filtered []ColumnDiff
	for _, diff := range diffs {
		ignore := false
		for _, pattern := range e.ignoreRules.Columns {
			parts := strings.Split(pattern, ".")
			if len(parts) == 2 {
				tablePattern, colPattern := parts[0], parts[1]
				tableMatch, _ := filepath.Match(tablePattern, tableName)
				colMatch, _ := filepath.Match(colPattern, diff.ColumnName)
				if tableMatch && colMatch {
					ignore = true
					break
				}
			}
		}
		if !ignore {
			filtered = append(filtered, diff)
		}
	}
	return filtered
}

// filterIgnoredProps 过滤忽略的属性变更
func (e *DiffEngine) filterIgnoredProps(props []PropertyDiff) []PropertyDiff {
	var filtered []PropertyDiff
	for _, prop := range props {
		ignore := false
		switch prop.Property {
		case "COMMENT":
			if e.ignoreRules.IgnoreComments {
				ignore = true
			}
		case "CHARSET":
			if e.ignoreRules.IgnoreCharset {
				ignore = true
			}
		case "COLLATION":
			if e.ignoreRules.IgnoreCollation {
				ignore = true
			}
		}
		if !ignore {
			filtered = append(filtered, prop)
		}
	}
	return filtered
}

// calculateStatistics 计算统计信息
func (e *DiffEngine) calculateStatistics(diff *SchemaDiff) DiffStatistics {
	stats := DiffStatistics{}

	for _, td := range diff.TableDiffs {
		switch td.DiffType {
		case DiffTypeAdded:
			stats.TablesAdded++
		case DiffTypeRemoved:
			stats.TablesRemoved++
		case DiffTypeModified:
			stats.TablesChanged++
		}
		switch td.Severity {
		case SeverityDanger:
			stats.DangerCount++
		case SeverityWarning:
			stats.WarningCount++
		case SeverityInfo:
			stats.InfoCount++
		}
	}

	for _, vd := range diff.ViewDiffs {
		switch vd.DiffType {
		case DiffTypeAdded:
			stats.ViewsAdded++
		case DiffTypeRemoved:
			stats.ViewsRemoved++
		case DiffTypeModified:
			stats.ViewsChanged++
		}
	}

	for _, pd := range diff.ProcDiffs {
		switch pd.DiffType {
		case DiffTypeAdded:
			stats.ProcsAdded++
		case DiffTypeRemoved:
			stats.ProcsRemoved++
		case DiffTypeModified:
			stats.ProcsChanged++
		}
	}

	for _, fd := range diff.FuncDiffs {
		switch fd.DiffType {
		case DiffTypeAdded:
			stats.FuncsAdded++
		case DiffTypeRemoved:
			stats.FuncsRemoved++
		case DiffTypeModified:
			stats.FuncsChanged++
		}
	}

	for _, td := range diff.TriggerDiffs {
		switch td.DiffType {
		case DiffTypeAdded:
			stats.TriggersAdded++
		case DiffTypeRemoved:
			stats.TriggersRemoved++
		case DiffTypeModified:
			stats.TriggersChanged++
		}
	}

	stats.TotalDiffs = len(diff.TableDiffs) + len(diff.ViewDiffs) +
		len(diff.ProcDiffs) + len(diff.FuncDiffs) + len(diff.TriggerDiffs)

	return stats
}
