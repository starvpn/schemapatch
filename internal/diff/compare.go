package diff

import (
	"fmt"
	"strings"

	"github.com/starvpn/schemapatch/internal/extractor"
)

// TableCompareOptions 表比较选项
type TableCompareOptions struct {
	IgnoreComments  bool
	IgnoreCharset   bool
	IgnoreCollation bool
}

// compareTables 比较表
func compareTables(source, target *extractor.TableSchema) *TableDiff {
	return compareTablesWithOptions(source, target, TableCompareOptions{})
}

// compareTablesWithOptions 带选项比较表
func compareTablesWithOptions(source, target *extractor.TableSchema, opts TableCompareOptions) *TableDiff {
	diff := &TableDiff{
		TableName: source.Name,
		DiffType:  DiffTypeModified,
		Severity:  SeverityInfo,
		OldTable:  target,
		NewTable:  source,
	}

	// 比较表属性
	if source.Engine != target.Engine {
		diff.TableProps = append(diff.TableProps, PropertyDiff{
			Property: "ENGINE",
			OldValue: target.Engine,
			NewValue: source.Engine,
		})
	}
	if !opts.IgnoreCharset && source.Charset != target.Charset {
		diff.TableProps = append(diff.TableProps, PropertyDiff{
			Property: "CHARSET",
			OldValue: target.Charset,
			NewValue: source.Charset,
		})
	}
	if !opts.IgnoreCollation && source.Collation != target.Collation {
		diff.TableProps = append(diff.TableProps, PropertyDiff{
			Property: "COLLATION",
			OldValue: target.Collation,
			NewValue: source.Collation,
		})
	}
	if !opts.IgnoreComments && source.Comment != target.Comment {
		diff.TableProps = append(diff.TableProps, PropertyDiff{
			Property: "COMMENT",
			OldValue: target.Comment,
			NewValue: source.Comment,
		})
	}

	// 比较列
	colOpts := ColumnCompareOptions{
		IgnoreComments:  opts.IgnoreComments,
		IgnoreCharset:   opts.IgnoreCharset,
		IgnoreCollation: opts.IgnoreCollation,
	}
	diff.ColumnDiffs = compareColumnsWithOptions(source.Columns, target.Columns, colOpts)

	// 比较索引
	diff.IndexDiffs = compareIndexes(source.Indexes, target.Indexes)

	// 比较外键
	diff.FKeyDiffs = compareForeignKeys(source.ForeignKeys, target.ForeignKeys)

	// 计算最高严重程度
	for _, cd := range diff.ColumnDiffs {
		if cd.Severity > diff.Severity {
			diff.Severity = cd.Severity
		}
	}
	for _, id := range diff.IndexDiffs {
		if id.Severity > diff.Severity {
			diff.Severity = id.Severity
		}
	}
	for _, fkd := range diff.FKeyDiffs {
		if fkd.Severity > diff.Severity {
			diff.Severity = fkd.Severity
		}
	}

	// 生成描述
	var changes []string
	if len(diff.ColumnDiffs) > 0 {
		changes = append(changes, fmt.Sprintf("%d列变更", len(diff.ColumnDiffs)))
	}
	if len(diff.IndexDiffs) > 0 {
		changes = append(changes, fmt.Sprintf("%d索引变更", len(diff.IndexDiffs)))
	}
	if len(diff.FKeyDiffs) > 0 {
		changes = append(changes, fmt.Sprintf("%d外键变更", len(diff.FKeyDiffs)))
	}
	if len(diff.TableProps) > 0 {
		changes = append(changes, fmt.Sprintf("%d属性变更", len(diff.TableProps)))
	}
	diff.Description = strings.Join(changes, ", ")

	return diff
}

// compareColumns 比较列
func compareColumns(sourceCols, targetCols []*extractor.ColumnSchema) []ColumnDiff {
	return compareColumnsWithOptions(sourceCols, targetCols, ColumnCompareOptions{})
}

// compareColumnsWithOptions 带选项比较列
func compareColumnsWithOptions(sourceCols, targetCols []*extractor.ColumnSchema, opts ColumnCompareOptions) []ColumnDiff {
	var diffs []ColumnDiff

	// 创建目标列映射
	targetMap := make(map[string]*extractor.ColumnSchema)
	for _, col := range targetCols {
		targetMap[col.Name] = col
	}

	// 创建源列映射
	sourceMap := make(map[string]*extractor.ColumnSchema)
	for _, col := range sourceCols {
		sourceMap[col.Name] = col
	}

	// 检查新增和修改的列
	for _, srcCol := range sourceCols {
		tgtCol, exists := targetMap[srcCol.Name]
		if !exists {
			// 新增列
			diffs = append(diffs, ColumnDiff{
				ColumnName: srcCol.Name,
				DiffType:   DiffTypeAdded,
				Severity:   SeverityInfo,
				NewColumn:  srcCol,
			})
		} else {
			// 比较列是否有变化
			colDiff := compareColumnWithOptions(srcCol, tgtCol, opts)
			if colDiff != nil {
				diffs = append(diffs, *colDiff)
			}
		}
	}

	// 检查删除的列
	for _, tgtCol := range targetCols {
		if _, exists := sourceMap[tgtCol.Name]; !exists {
			diffs = append(diffs, ColumnDiff{
				ColumnName: tgtCol.Name,
				DiffType:   DiffTypeRemoved,
				Severity:   SeverityDanger,
				OldColumn:  tgtCol,
				RiskNote:   "删除列将导致数据丢失",
			})
		}
	}

	return diffs
}

// ColumnCompareOptions 列比较选项
type ColumnCompareOptions struct {
	IgnoreComments  bool
	IgnoreCharset   bool
	IgnoreCollation bool
}

// compareColumn 比较单个列
func compareColumn(source, target *extractor.ColumnSchema) *ColumnDiff {
	return compareColumnWithOptions(source, target, ColumnCompareOptions{})
}

// compareColumnWithOptions 带选项比较单个列
func compareColumnWithOptions(source, target *extractor.ColumnSchema, opts ColumnCompareOptions) *ColumnDiff {
	var changes []PropertyDiff

	// 比较列类型
	if source.ColumnType != target.ColumnType {
		changes = append(changes, PropertyDiff{
			Property: "类型",
			OldValue: target.ColumnType,
			NewValue: source.ColumnType,
		})
	}

	// 比较是否可空
	if source.IsNullable != target.IsNullable {
		oldVal := "NOT NULL"
		newVal := "NOT NULL"
		if target.IsNullable {
			oldVal = "NULL"
		}
		if source.IsNullable {
			newVal = "NULL"
		}
		changes = append(changes, PropertyDiff{
			Property: "可空",
			OldValue: oldVal,
			NewValue: newVal,
		})
	}

	// 比较默认值
	srcDefault := ""
	tgtDefault := ""
	if source.DefaultValue != nil {
		srcDefault = *source.DefaultValue
	}
	if target.DefaultValue != nil {
		tgtDefault = *target.DefaultValue
	}
	if srcDefault != tgtDefault {
		changes = append(changes, PropertyDiff{
			Property: "默认值",
			OldValue: tgtDefault,
			NewValue: srcDefault,
		})
	}

	// 比较注释（可选忽略）
	if !opts.IgnoreComments && source.Comment != target.Comment {
		changes = append(changes, PropertyDiff{
			Property: "注释",
			OldValue: target.Comment,
			NewValue: source.Comment,
		})
	}

	// 比较自增
	if source.IsAutoIncr != target.IsAutoIncr {
		oldVal := "否"
		newVal := "否"
		if target.IsAutoIncr {
			oldVal = "是"
		}
		if source.IsAutoIncr {
			newVal = "是"
		}
		changes = append(changes, PropertyDiff{
			Property: "自增",
			OldValue: oldVal,
			NewValue: newVal,
		})
	}

	// 比较字符集（可选忽略）
	if !opts.IgnoreCharset && source.CharsetName != target.CharsetName {
		changes = append(changes, PropertyDiff{
			Property: "字符集",
			OldValue: target.CharsetName,
			NewValue: source.CharsetName,
		})
	}

	// 比较排序规则（可选忽略）
	if !opts.IgnoreCollation && source.CollationName != target.CollationName {
		changes = append(changes, PropertyDiff{
			Property: "排序规则",
			OldValue: target.CollationName,
			NewValue: source.CollationName,
		})
	}

	if len(changes) == 0 {
		return nil
	}

	diff := &ColumnDiff{
		ColumnName: source.Name,
		DiffType:   DiffTypeModified,
		Severity:   SeverityInfo,
		OldColumn:  target,
		NewColumn:  source,
		Changes:    changes,
	}

	// 评估风险
	diff.Severity, diff.RiskNote = assessColumnRisk(source, target, changes)

	return diff
}

// assessColumnRisk 评估列变更风险
func assessColumnRisk(source, target *extractor.ColumnSchema, changes []PropertyDiff) (DiffSeverity, string) {
	severity := SeverityInfo
	var risks []string

	for _, change := range changes {
		switch change.Property {
		case "类型":
			// 检查类型收缩
			if isTypeShrink(target.ColumnType, source.ColumnType) {
				severity = SeverityDanger
				risks = append(risks, "类型收缩可能导致数据截断")
			} else if isTypeChange(target.ColumnType, source.ColumnType) {
				if severity < SeverityWarning {
					severity = SeverityWarning
				}
				risks = append(risks, "类型变更可能影响数据")
			}
		case "可空":
			if !source.IsNullable && target.IsNullable {
				// NULL -> NOT NULL
				if severity < SeverityWarning {
					severity = SeverityWarning
				}
				risks = append(risks, "已有NULL值需要处理")
			}
		}
	}

	return severity, strings.Join(risks, "; ")
}

// isTypeShrink 检查是否是类型收缩
func isTypeShrink(oldType, newType string) bool {
	// 简单的类型收缩检测
	oldLen := extractTypeLength(oldType)
	newLen := extractTypeLength(newType)

	if oldLen > 0 && newLen > 0 && newLen < oldLen {
		return true
	}

	// 检查数值类型降级
	numericOrder := map[string]int{
		"tinyint": 1, "smallint": 2, "mediumint": 3,
		"int": 4, "bigint": 5,
	}

	oldBase := extractBaseType(oldType)
	newBase := extractBaseType(newType)

	if oldOrder, ok1 := numericOrder[oldBase]; ok1 {
		if newOrder, ok2 := numericOrder[newBase]; ok2 {
			if newOrder < oldOrder {
				return true
			}
		}
	}

	return false
}

// isTypeChange 检查是否是类型变更
func isTypeChange(oldType, newType string) bool {
	oldBase := extractBaseType(oldType)
	newBase := extractBaseType(newType)
	return oldBase != newBase
}

// extractTypeLength 提取类型长度
func extractTypeLength(colType string) int {
	// 解析 varchar(255) -> 255
	start := strings.Index(colType, "(")
	end := strings.Index(colType, ")")
	if start > 0 && end > start {
		var length int
		fmt.Sscanf(colType[start+1:end], "%d", &length)
		return length
	}
	return 0
}

// extractBaseType 提取基础类型
func extractBaseType(colType string) string {
	// varchar(255) -> varchar
	// int(11) unsigned -> int
	colType = strings.ToLower(colType)
	if idx := strings.Index(colType, "("); idx > 0 {
		colType = colType[:idx]
	}
	if idx := strings.Index(colType, " "); idx > 0 {
		colType = colType[:idx]
	}
	return colType
}

// compareIndexes 比较索引
func compareIndexes(sourceIdxs, targetIdxs map[string]*extractor.IndexSchema) []IndexDiff {
	var diffs []IndexDiff

	// 检查新增和修改的索引
	for name, srcIdx := range sourceIdxs {
		tgtIdx, exists := targetIdxs[name]
		if !exists {
			diffs = append(diffs, IndexDiff{
				IndexName:   name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityInfo,
				NewIndex:    srcIdx,
				Description: fmt.Sprintf("新增%s索引", srcIdx.Type.String()),
			})
		} else {
			// 比较索引是否有变化
			if !indexEquals(srcIdx, tgtIdx) {
				diffs = append(diffs, IndexDiff{
					IndexName:   name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldIndex:    tgtIdx,
					NewIndex:    srcIdx,
					Description: "索引定义已变更",
				})
			}
		}
	}

	// 检查删除的索引
	for name, tgtIdx := range targetIdxs {
		if _, exists := sourceIdxs[name]; !exists {
			diffs = append(diffs, IndexDiff{
				IndexName:   name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldIndex:    tgtIdx,
				Description: fmt.Sprintf("删除%s索引", tgtIdx.Type.String()),
			})
		}
	}

	return diffs
}

// indexEquals 检查索引是否相等
func indexEquals(a, b *extractor.IndexSchema) bool {
	if a.Type != b.Type || a.IsUnique != b.IsUnique || a.IsPrimary != b.IsPrimary {
		return false
	}
	if len(a.Columns) != len(b.Columns) {
		return false
	}
	for i, col := range a.Columns {
		if col.Name != b.Columns[i].Name || col.SeqInIdx != b.Columns[i].SeqInIdx {
			return false
		}
		// 比较前缀长度
		if (col.SubPart == nil) != (b.Columns[i].SubPart == nil) {
			return false
		}
		if col.SubPart != nil && *col.SubPart != *b.Columns[i].SubPart {
			return false
		}
	}
	return true
}

// compareForeignKeys 比较外键
func compareForeignKeys(sourceFKs, targetFKs map[string]*extractor.ForeignKey) []ForeignKeyDiff {
	var diffs []ForeignKeyDiff

	// 检查新增和修改的外键
	for name, srcFK := range sourceFKs {
		tgtFK, exists := targetFKs[name]
		if !exists {
			diffs = append(diffs, ForeignKeyDiff{
				FKeyName:    name,
				DiffType:    DiffTypeAdded,
				Severity:    SeverityWarning,
				NewFKey:     srcFK,
				Description: fmt.Sprintf("新增外键引用 %s", srcFK.RefTable),
			})
		} else {
			// 比较外键是否有变化
			if !foreignKeyEquals(srcFK, tgtFK) {
				diffs = append(diffs, ForeignKeyDiff{
					FKeyName:    name,
					DiffType:    DiffTypeModified,
					Severity:    SeverityWarning,
					OldFKey:     tgtFK,
					NewFKey:     srcFK,
					Description: "外键定义已变更",
				})
			}
		}
	}

	// 检查删除的外键
	for name, tgtFK := range targetFKs {
		if _, exists := sourceFKs[name]; !exists {
			diffs = append(diffs, ForeignKeyDiff{
				FKeyName:    name,
				DiffType:    DiffTypeRemoved,
				Severity:    SeverityWarning,
				OldFKey:     tgtFK,
				Description: fmt.Sprintf("删除外键引用 %s", tgtFK.RefTable),
			})
		}
	}

	return diffs
}

// foreignKeyEquals 检查外键是否相等
func foreignKeyEquals(a, b *extractor.ForeignKey) bool {
	if a.RefTable != b.RefTable || a.OnDelete != b.OnDelete || a.OnUpdate != b.OnUpdate {
		return false
	}
	if len(a.Columns) != len(b.Columns) || len(a.RefColumns) != len(b.RefColumns) {
		return false
	}
	for i := range a.Columns {
		if a.Columns[i] != b.Columns[i] || a.RefColumns[i] != b.RefColumns[i] {
			return false
		}
	}
	return true
}
