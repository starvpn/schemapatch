package diff

import (
	"fmt"
	"strings"
)

// RiskLevel 风险级别
type RiskLevel int

const (
	RiskLow    RiskLevel = iota // 低风险
	RiskMedium                  // 中风险
	RiskHigh                    // 高风险
)

func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "低风险"
	case RiskMedium:
		return "中风险"
	case RiskHigh:
		return "高风险"
	default:
		return "未知"
	}
}

// RiskAssessment 风险评估结果
type RiskAssessment struct {
	Level       RiskLevel `json:"level"`
	Score       int       `json:"score"`        // 0-100
	Description string    `json:"description"`
	Warnings    []string  `json:"warnings"`
	Suggestions []string  `json:"suggestions"`
}

// RiskAssessor 风险评估器
type RiskAssessor struct{}

// NewRiskAssessor 创建风险评估器
func NewRiskAssessor() *RiskAssessor {
	return &RiskAssessor{}
}

// Assess 评估Schema差异的风险
func (r *RiskAssessor) Assess(diff *SchemaDiff) *RiskAssessment {
	assessment := &RiskAssessment{
		Level:       RiskLow,
		Score:       0,
		Warnings:    []string{},
		Suggestions: []string{},
	}

	// 评估表变更风险
	for _, td := range diff.TableDiffs {
		r.assessTableDiff(&td, assessment)
	}

	// 评估视图变更风险
	for _, vd := range diff.ViewDiffs {
		r.assessViewDiff(&vd, assessment)
	}

	// 评估存储过程变更风险
	for _, pd := range diff.ProcDiffs {
		r.assessProcedureDiff(&pd, assessment)
	}

	// 评估触发器变更风险
	for _, td := range diff.TriggerDiffs {
		r.assessTriggerDiff(&td, assessment)
	}

	// 计算最终风险级别
	if assessment.Score >= 70 {
		assessment.Level = RiskHigh
	} else if assessment.Score >= 40 {
		assessment.Level = RiskMedium
	} else {
		assessment.Level = RiskLow
	}

	// 生成描述
	assessment.Description = r.generateDescription(assessment, diff)

	return assessment
}

// assessTableDiff 评估表差异风险
func (r *RiskAssessor) assessTableDiff(td *TableDiff, assessment *RiskAssessment) {
	switch td.DiffType {
	case DiffTypeRemoved:
		// 删除表是高风险操作
		assessment.Score += 30
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 删除表 `%s` 将导致所有数据永久丢失", td.TableName))
		assessment.Suggestions = append(assessment.Suggestions,
			fmt.Sprintf("建议在删除表 `%s` 前先备份数据", td.TableName))

	case DiffTypeModified:
		// 评估列变更风险
		for _, cd := range td.ColumnDiffs {
			r.assessColumnDiff(td.TableName, &cd, assessment)
		}

		// 评估索引变更风险
		for _, id := range td.IndexDiffs {
			r.assessIndexDiff(td.TableName, &id, assessment)
		}

		// 评估外键变更风险
		for _, fkd := range td.FKeyDiffs {
			r.assessForeignKeyDiff(td.TableName, &fkd, assessment)
		}
	}
}

// assessColumnDiff 评估列差异风险
func (r *RiskAssessor) assessColumnDiff(tableName string, cd *ColumnDiff, assessment *RiskAssessment) {
	switch cd.DiffType {
	case DiffTypeRemoved:
		// 删除列是高风险操作
		assessment.Score += 20
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 删除列 `%s`.`%s` 将导致该列数据丢失", tableName, cd.ColumnName))

	case DiffTypeModified:
		// 检查危险的修改
		for _, change := range cd.Changes {
			switch change.Property {
			case "类型":
				if isTypeShrink(change.OldValue, change.NewValue) {
					assessment.Score += 15
					assessment.Warnings = append(assessment.Warnings,
						fmt.Sprintf("⚠️ 列 `%s`.`%s` 类型从 %s 缩小到 %s，可能导致数据截断",
							tableName, cd.ColumnName, change.OldValue, change.NewValue))
				} else if isTypeChange(change.OldValue, change.NewValue) {
					assessment.Score += 10
					assessment.Warnings = append(assessment.Warnings,
						fmt.Sprintf("⚠️ 列 `%s`.`%s` 类型从 %s 变更为 %s",
							tableName, cd.ColumnName, change.OldValue, change.NewValue))
				}

			case "可空":
				if change.NewValue == "NOT NULL" && change.OldValue == "NULL" {
					assessment.Score += 10
					assessment.Warnings = append(assessment.Warnings,
						fmt.Sprintf("⚠️ 列 `%s`.`%s` 从可空变为非空，需要处理现有NULL值",
							tableName, cd.ColumnName))
					assessment.Suggestions = append(assessment.Suggestions,
						fmt.Sprintf("在修改列 `%s`.`%s` 为 NOT NULL 前，请先更新现有的NULL值",
							tableName, cd.ColumnName))
				}
			}
		}
	}
}

// assessIndexDiff 评估索引差异风险
func (r *RiskAssessor) assessIndexDiff(tableName string, id *IndexDiff, assessment *RiskAssessment) {
	switch id.DiffType {
	case DiffTypeRemoved:
		if id.OldIndex != nil && id.OldIndex.IsPrimary {
			assessment.Score += 20
			assessment.Warnings = append(assessment.Warnings,
				fmt.Sprintf("⚠️ 删除表 `%s` 的主键", tableName))
		} else if id.OldIndex != nil && id.OldIndex.IsUnique {
			assessment.Score += 10
			assessment.Warnings = append(assessment.Warnings,
				fmt.Sprintf("⚠️ 删除表 `%s` 的唯一索引 `%s`", tableName, id.IndexName))
		}

	case DiffTypeAdded:
		// 添加索引通常是安全的，但对大表可能耗时
		assessment.Suggestions = append(assessment.Suggestions,
			fmt.Sprintf("添加索引 `%s`.`%s` 可能在大表上耗时较长，建议在低峰期执行",
				tableName, id.IndexName))
	}
}

// assessForeignKeyDiff 评估外键差异风险
func (r *RiskAssessor) assessForeignKeyDiff(tableName string, fkd *ForeignKeyDiff, assessment *RiskAssessment) {
	switch fkd.DiffType {
	case DiffTypeAdded:
		if fkd.NewFKey != nil {
			assessment.Score += 5
			assessment.Warnings = append(assessment.Warnings,
				fmt.Sprintf("⚠️ 添加外键 `%s`.`%s` 可能因现有数据不符合约束而失败",
					tableName, fkd.FKeyName))
			assessment.Suggestions = append(assessment.Suggestions,
				fmt.Sprintf("在添加外键前，请确保 `%s` 中所有值都在 `%s` 中存在",
					tableName, fkd.NewFKey.RefTable))
		}
	}
}

// assessViewDiff 评估视图差异风险
func (r *RiskAssessor) assessViewDiff(vd *ViewDiff, assessment *RiskAssessment) {
	switch vd.DiffType {
	case DiffTypeRemoved:
		assessment.Score += 5
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 删除视图 `%s`", vd.ViewName))
	case DiffTypeModified:
		assessment.Score += 3
	}
}

// assessProcedureDiff 评估存储过程差异风险
func (r *RiskAssessor) assessProcedureDiff(pd *ProcedureDiff, assessment *RiskAssessment) {
	switch pd.DiffType {
	case DiffTypeRemoved:
		assessment.Score += 10
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 删除存储过程 `%s`，可能影响依赖它的应用", pd.ProcName))
	case DiffTypeModified:
		assessment.Score += 5
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 修改存储过程 `%s`，请确认修改不会影响调用方", pd.ProcName))
	}
}

// assessTriggerDiff 评估触发器差异风险
func (r *RiskAssessor) assessTriggerDiff(td *TriggerDiff, assessment *RiskAssessment) {
	switch td.DiffType {
	case DiffTypeRemoved:
		assessment.Score += 10
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 删除触发器 `%s`，可能影响数据一致性逻辑", td.TriggerName))
	case DiffTypeModified:
		assessment.Score += 8
		assessment.Warnings = append(assessment.Warnings,
			fmt.Sprintf("⚠️ 修改触发器 `%s`", td.TriggerName))
	}
}

// generateDescription 生成风险描述
func (r *RiskAssessor) generateDescription(assessment *RiskAssessment, diff *SchemaDiff) string {
	var parts []string

	// 统计危险操作
	dangerOps := 0
	warningOps := 0

	for _, td := range diff.TableDiffs {
		if td.Severity == SeverityDanger {
			dangerOps++
		} else if td.Severity == SeverityWarning {
			warningOps++
		}
	}

	if dangerOps > 0 {
		parts = append(parts, fmt.Sprintf("包含 %d 个危险操作", dangerOps))
	}
	if warningOps > 0 {
		parts = append(parts, fmt.Sprintf("%d 个警告", warningOps))
	}

	if len(parts) == 0 {
		return "变更风险较低，可以安全执行"
	}

	return strings.Join(parts, "，") + "，建议在验证环境测试后再执行"
}

// GetRiskIcon 获取风险图标
func GetRiskIcon(level RiskLevel) string {
	switch level {
	case RiskHigh:
		return "🔴"
	case RiskMedium:
		return "🟡"
	case RiskLow:
		return "🟢"
	default:
		return "⚪"
	}
}

// GetSeverityIcon 获取严重程度图标
func GetSeverityIcon(severity DiffSeverity) string {
	switch severity {
	case SeverityDanger:
		return "🔴"
	case SeverityWarning:
		return "🟡"
	case SeverityInfo:
		return "🟢"
	default:
		return "⚪"
	}
}

// GetDiffTypeIcon 获取差异类型图标
func GetDiffTypeIcon(diffType DiffType) string {
	switch diffType {
	case DiffTypeAdded:
		return "➕"
	case DiffTypeRemoved:
		return "➖"
	case DiffTypeModified:
		return "✏️"
	default:
		return "❓"
	}
}
