package gui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/starvpn/schemapatch/internal/config"
	"github.com/starvpn/schemapatch/internal/diff"
	"github.com/starvpn/schemapatch/internal/docker"
	"github.com/starvpn/schemapatch/internal/extractor"
	"github.com/starvpn/schemapatch/internal/sqlgen"
	"go.uber.org/zap"
)

// MainWindow 主窗口
type MainWindow struct {
	window fyne.Window
	store  *config.Store
	app    fyne.App

	// 当前状态
	sourceSchema *extractor.DatabaseSchema
	targetSchema *extractor.DatabaseSchema
	schemaDiff   *diff.SchemaDiff
	script       *sqlgen.MigrationScript

	// UI组件
	sourceEnvPanel *EnvPanel
	targetEnvPanel *EnvPanel
	diffTree       *widget.Tree
	sqlPreview     *widget.Entry
	statusBar      *widget.Label
	progressBar    *widget.ProgressBar

	// 忽略选项
	ignoreComments  *widget.Check
	ignoreCharset   *widget.Check
	ignoreCollation *widget.Check

	// 按钮
	compareBtn  *widget.Button
	generateBtn *widget.Button
	validateBtn *widget.Button
	exportBtn   *widget.Button
}

// NewMainWindow 创建主窗口
func NewMainWindow(app fyne.App, store *config.Store) *MainWindow {
	window := app.NewWindow("SchemaPatch - MySQL数据库对比工具")
	window.Resize(fyne.NewSize(1200, 800))
	window.CenterOnScreen()

	mw := &MainWindow{
		window: window,
		store:  store,
		app:    app,
	}

	mw.buildUI()
	return mw
}

// buildUI 构建UI
func (mw *MainWindow) buildUI() {
	// 创建环境配置面板
	mw.sourceEnvPanel = NewEnvPanel("开发环境 (Source)", ColorGreen, config.EnvTypeDev)
	mw.targetEnvPanel = NewEnvPanel("生产环境 (Target)", ColorPeach, config.EnvTypeProd)

	// 忽略选项（先创建，loadConfig 需要用到）
	mw.ignoreComments = widget.NewCheck("忽略注释差异", func(checked bool) {
		mw.updateIgnoreRules()
	})
	mw.ignoreComments.SetChecked(true) // 默认忽略注释

	mw.ignoreCharset = widget.NewCheck("忽略字符集差异", func(checked bool) {
		mw.updateIgnoreRules()
	})
	mw.ignoreCharset.SetChecked(true) // 默认忽略字符集

	mw.ignoreCollation = widget.NewCheck("忽略排序规则差异", func(checked bool) {
		mw.updateIgnoreRules()
	})
	mw.ignoreCollation.SetChecked(true) // 默认忽略排序规则

	// 设置变更回调 - 自动保存配置
	mw.sourceEnvPanel.SetOnChanged(mw.saveConfig)
	mw.targetEnvPanel.SetOnChanged(mw.saveConfig)

	// 加载项目配置（在所有 UI 组件创建后）
	mw.loadConfig()

	// 环境配置区域
	envContainer := container.NewGridWithColumns(2,
		mw.sourceEnvPanel.Container(),
		mw.targetEnvPanel.Container(),
	)

	// 对比按钮
	mw.compareBtn = widget.NewButtonWithIcon("开始对比", theme.SearchIcon(), mw.onCompare)
	mw.compareBtn.Importance = widget.HighImportance

	optionsRow := container.NewHBox(
		widget.NewLabel("对比选项:"),
		mw.ignoreComments,
		mw.ignoreCharset,
		mw.ignoreCollation,
	)

	compareRow := container.NewHBox(
		layout.NewSpacer(),
		mw.compareBtn,
		layout.NewSpacer(),
	)

	// 差异树
	mw.diffTree = mw.createDiffTree()
	diffCard := widget.NewCard("差异结果", "", container.NewScroll(mw.diffTree))

	// SQL预览
	mw.sqlPreview = widget.NewMultiLineEntry()
	mw.sqlPreview.SetPlaceHolder("-- 升级SQL将显示在这里...")
	mw.sqlPreview.Wrapping = fyne.TextWrapWord
	sqlCard := widget.NewCard("升级脚本预览", "", container.NewScroll(mw.sqlPreview))

	// 右侧面板
	rightPanel := container.NewVSplit(
		diffCard,
		sqlCard,
	)
	rightPanel.SetOffset(0.5)

	// 操作按钮
	mw.generateBtn = widget.NewButtonWithIcon("生成脚本", theme.DocumentCreateIcon(), mw.onGenerate)
	mw.generateBtn.Disable()

	mw.validateBtn = widget.NewButtonWithIcon("Docker验证", theme.ConfirmIcon(), mw.onValidate)
	mw.validateBtn.Disable()

	mw.exportBtn = widget.NewButtonWithIcon("导出脚本", theme.DocumentSaveIcon(), mw.onExport)
	mw.exportBtn.Disable()

	actionRow := container.NewHBox(
		mw.generateBtn,
		mw.validateBtn,
		mw.exportBtn,
		layout.NewSpacer(),
	)

	// 状态栏
	mw.statusBar = widget.NewLabel("就绪")
	mw.progressBar = widget.NewProgressBar()
	mw.progressBar.Hide()

	statusRow := container.NewBorder(nil, nil, mw.statusBar, mw.progressBar)

	// 主布局
	topSection := container.NewVBox(
		envContainer,
		optionsRow,
		compareRow,
	)

	mainContent := container.NewBorder(
		topSection,
		container.NewVBox(actionRow, statusRow),
		nil,
		nil,
		rightPanel,
	)

	mw.window.SetContent(mainContent)
}

// createDiffTree 创建差异树
func (mw *MainWindow) createDiffTree() *widget.Tree {
	tree := widget.NewTree(
		// childUIDs
		func(uid string) []string {
			if mw.schemaDiff == nil {
				return []string{}
			}

			if uid == "" {
				// 根节点
				var roots []string
				if len(mw.schemaDiff.TableDiffs) > 0 {
					roots = append(roots, "tables")
				}
				if len(mw.schemaDiff.ViewDiffs) > 0 {
					roots = append(roots, "views")
				}
				if len(mw.schemaDiff.ProcDiffs) > 0 {
					roots = append(roots, "procedures")
				}
				if len(mw.schemaDiff.FuncDiffs) > 0 {
					roots = append(roots, "functions")
				}
				if len(mw.schemaDiff.TriggerDiffs) > 0 {
					roots = append(roots, "triggers")
				}
				return roots
			}

			switch uid {
			case "tables":
				var items []string
				for _, td := range mw.schemaDiff.TableDiffs {
					items = append(items, "table:"+td.TableName)
				}
				return items
			case "views":
				var items []string
				for _, vd := range mw.schemaDiff.ViewDiffs {
					items = append(items, "view:"+vd.ViewName)
				}
				return items
			case "procedures":
				var items []string
				for _, pd := range mw.schemaDiff.ProcDiffs {
					items = append(items, "proc:"+pd.ProcName)
				}
				return items
			case "functions":
				var items []string
				for _, fd := range mw.schemaDiff.FuncDiffs {
					items = append(items, "func:"+fd.FuncName)
				}
				return items
			case "triggers":
				var items []string
				for _, td := range mw.schemaDiff.TriggerDiffs {
					items = append(items, "trigger:"+td.TriggerName)
				}
				return items
			}

			return []string{}
		},
		// isBranch
		func(uid string) bool {
			return uid == "" || uid == "tables" || uid == "views" || uid == "procedures" || uid == "functions" || uid == "triggers"
		},
		// create
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		// update
		func(uid string, branch bool, obj fyne.CanvasObject) {
			label := obj.(*widget.Label)

			switch uid {
			case "tables":
				label.SetText(fmt.Sprintf("📋 表 (%d)", len(mw.schemaDiff.TableDiffs)))
			case "views":
				label.SetText(fmt.Sprintf("📊 视图 (%d)", len(mw.schemaDiff.ViewDiffs)))
			case "procedures":
				label.SetText(fmt.Sprintf("⚙️ 存储过程 (%d)", len(mw.schemaDiff.ProcDiffs)))
			case "functions":
				label.SetText(fmt.Sprintf("🔧 函数 (%d)", len(mw.schemaDiff.FuncDiffs)))
			case "triggers":
				label.SetText(fmt.Sprintf("⚡ 触发器 (%d)", len(mw.schemaDiff.TriggerDiffs)))
			default:
				// 具体项
				if len(uid) > 6 && uid[:6] == "table:" {
					tableName := uid[6:]
					for _, td := range mw.schemaDiff.TableDiffs {
						if td.TableName == tableName {
							icon := diff.GetSeverityIcon(td.Severity)
							typeIcon := diff.GetDiffTypeIcon(td.DiffType)
							label.SetText(fmt.Sprintf("%s %s %s - %s", icon, typeIcon, tableName, td.Description))
							break
						}
					}
				} else if len(uid) > 5 && uid[:5] == "view:" {
					viewName := uid[5:]
					for _, vd := range mw.schemaDiff.ViewDiffs {
						if vd.ViewName == viewName {
							icon := diff.GetSeverityIcon(vd.Severity)
							typeIcon := diff.GetDiffTypeIcon(vd.DiffType)
							label.SetText(fmt.Sprintf("%s %s %s", icon, typeIcon, viewName))
							break
						}
					}
				} else if len(uid) > 5 && uid[:5] == "proc:" {
					procName := uid[5:]
					for _, pd := range mw.schemaDiff.ProcDiffs {
						if pd.ProcName == procName {
							icon := diff.GetSeverityIcon(pd.Severity)
							typeIcon := diff.GetDiffTypeIcon(pd.DiffType)
							label.SetText(fmt.Sprintf("%s %s %s", icon, typeIcon, procName))
							break
						}
					}
				} else if len(uid) > 5 && uid[:5] == "func:" {
					funcName := uid[5:]
					for _, fd := range mw.schemaDiff.FuncDiffs {
						if fd.FuncName == funcName {
							icon := diff.GetSeverityIcon(fd.Severity)
							typeIcon := diff.GetDiffTypeIcon(fd.DiffType)
							label.SetText(fmt.Sprintf("%s %s %s", icon, typeIcon, funcName))
							break
						}
					}
				} else if len(uid) > 8 && uid[:8] == "trigger:" {
					triggerName := uid[8:]
					for _, td := range mw.schemaDiff.TriggerDiffs {
						if td.TriggerName == triggerName {
							icon := diff.GetSeverityIcon(td.Severity)
							typeIcon := diff.GetDiffTypeIcon(td.DiffType)
							label.SetText(fmt.Sprintf("%s %s %s", icon, typeIcon, triggerName))
							break
						}
					}
				}
			}
		},
	)

	return tree
}

// onCompare 对比按钮点击
func (mw *MainWindow) onCompare() {
	mw.setStatus("正在对比...")
	mw.compareBtn.Disable()
	mw.progressBar.Show()
	mw.progressBar.SetValue(0)

	go func() {
		ctx := context.Background()

		// 获取环境配置
		sourceEnv := mw.sourceEnvPanel.GetEnvironment()
		targetEnv := mw.targetEnvPanel.GetEnvironment()

		if sourceEnv == nil || targetEnv == nil {
			mw.showError("请配置数据库环境")
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}

		// 提取源Schema
		mw.setStatus("正在连接开发环境...")
		mw.progressBar.SetValue(0.1)

		sourceExtractor, err := extractor.NewMySQLExtractor(sourceEnv)
		if err != nil {
			mw.showError("创建源提取器失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}
		defer sourceExtractor.Close()

		if err := sourceExtractor.Connect(ctx); err != nil {
			mw.showError("连接开发环境失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}

		mw.setStatus("正在提取开发环境Schema...")
		mw.progressBar.SetValue(0.3)

		sourceSchema, err := sourceExtractor.ExtractSchema(ctx, extractor.DefaultExtractOptions())
		if err != nil {
			mw.showError("提取开发环境Schema失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}
		mw.sourceSchema = sourceSchema

		// 提取目标Schema
		mw.setStatus("正在连接生产环境...")
		mw.progressBar.SetValue(0.5)

		targetExtractor, err := extractor.NewMySQLExtractor(targetEnv)
		if err != nil {
			mw.showError("创建目标提取器失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}
		defer targetExtractor.Close()

		if err := targetExtractor.Connect(ctx); err != nil {
			mw.showError("连接生产环境失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}

		mw.setStatus("正在提取生产环境Schema...")
		mw.progressBar.SetValue(0.7)

		targetSchema, err := targetExtractor.ExtractSchema(ctx, extractor.DefaultExtractOptions())
		if err != nil {
			mw.showError("提取生产环境Schema失败: " + err.Error())
			mw.compareBtn.Enable()
			mw.progressBar.Hide()
			return
		}
		mw.targetSchema = targetSchema

		// 执行对比
		mw.setStatus("正在分析差异...")
		mw.progressBar.SetValue(0.9)

		project := mw.store.GetActiveProject()
		var ignoreRules config.IgnoreConfig
		if project != nil {
			ignoreRules = project.IgnoreRules
		}

		diffEngine := diff.NewDiffEngine(ignoreRules)
		mw.schemaDiff = diffEngine.Compare(sourceSchema, targetSchema)

		mw.progressBar.SetValue(1.0)

		// 更新状态
		stats := mw.schemaDiff.Statistics
		statusText := fmt.Sprintf("对比完成 | 差异: %d项 | 🔴%d 🟡%d 🟢%d",
			stats.TotalDiffs, stats.DangerCount, stats.WarningCount, stats.InfoCount)
		mw.setStatus(statusText)

		// 启用按钮
		if mw.schemaDiff.HasDiff() {
			mw.generateBtn.Enable()
		}

		mw.compareBtn.Enable()
		mw.progressBar.Hide()

		// 强制刷新UI（在goroutine中更新UI后需要显式刷新）
		mw.diffTree.Refresh()
		mw.window.Content().Refresh()
	}()
}

// onGenerate 生成脚本按钮点击
func (mw *MainWindow) onGenerate() {
	if mw.schemaDiff == nil {
		return
	}

	mw.setStatus("正在生成SQL脚本...")

	generator := sqlgen.NewMySQLGenerator()
	options := sqlgen.DefaultGenerateOptions()
	options.AddComments = true

	script, err := generator.Generate(mw.schemaDiff, options)
	if err != nil {
		mw.showError("生成脚本失败: " + err.Error())
		return
	}

	mw.script = script
	mw.sqlPreview.SetText(script.UpSQL)

	mw.setStatus(fmt.Sprintf("脚本生成完成 | 语句数: %d", len(script.Statements)))
	mw.validateBtn.Enable()
	mw.exportBtn.Enable()
}

// onValidate Docker验证按钮点击
func (mw *MainWindow) onValidate() {
	if mw.script == nil || mw.targetSchema == nil {
		return
	}

	// 创建验证对话框
	progress := widget.NewProgressBar()
	logText := widget.NewMultiLineEntry()
	logText.Wrapping = fyne.TextWrapWord

	// 创建固定大小的滚动区域
	logScroll := container.NewScroll(logText)
	logScroll.SetMinSize(fyne.NewSize(850, 400))

	content := container.NewVBox(
		widget.NewLabel("MySQL版本: 8.0"),
		progress,
		widget.NewCard("执行日志", "", logScroll),
	)

	d := dialog.NewCustom("🐳 Docker验证", "关闭", content, mw.window)
	d.Resize(fyne.NewSize(900, 600))
	d.Show()

	// 开始验证
	go func() {
		ctx := context.Background()
		validator := docker.NewValidator()
		defer validator.Cleanup(ctx)

		options := docker.DefaultValidationOptions()

		// sourceSchema: 开发环境（升级目标）, targetSchema: 生产环境（当前状态）
		result, err := validator.Validate(ctx, mw.sourceSchema, mw.targetSchema, mw.script, options,
			func(step, total int, message string, stepErr error) {
				progress.SetValue(float64(step) / float64(total))

				timestamp := time.Now().Format("15:04:05")
				status := "✅"
				if stepErr != nil {
					status = "❌"
				}
				logLine := fmt.Sprintf("[%s] %s %s\n", timestamp, status, message)

				logText.SetText(logText.Text + logLine)
				logText.Refresh()
				logScroll.ScrollToBottom()
			})

		if err != nil {
			logText.SetText(logText.Text + fmt.Sprintf("\n❌ 验证失败: %s\n", err.Error()))
		} else if result.Success {
			logText.SetText(logText.Text + fmt.Sprintf("\n✅ 验证成功! 耗时: %v\n", result.ExecutionTime))
		} else {
			logText.SetText(logText.Text + fmt.Sprintf("\n❌ 验证失败\n错误: %v\n", result.Errors))
		}
		logText.Refresh()
		logScroll.ScrollToBottom()
	}()
}

// onExport 导出脚本按钮点击
func (mw *MainWindow) onExport() {
	if mw.script == nil {
		return
	}

	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			mw.showError(err.Error())
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()

		_, err = writer.Write([]byte(mw.script.UpSQL))
		if err != nil {
			mw.showError("保存失败: " + err.Error())
			return
		}

		mw.setStatus("脚本已导出")
	}, mw.window)
}

// setStatus 设置状态栏文本
func (mw *MainWindow) setStatus(text string) {
	mw.statusBar.SetText(text)
	zap.S().Info(text)
}

// showError 显示错误对话框
func (mw *MainWindow) showError(message string) {
	mw.setStatus("错误: " + message)
	dialog.ShowError(fmt.Errorf(message), mw.window)
}

// Show 显示窗口
func (mw *MainWindow) Show() {
	mw.window.Show()
}

// loadConfig 加载配置
func (mw *MainWindow) loadConfig() {
	project := mw.store.GetActiveProject()
	if project == nil {
		// 创建默认项目
		project = &config.Project{
			ID:   "default",
			Name: "默认项目",
			Environments: []config.Environment{
				{ID: "dev", Name: "开发环境", Type: config.EnvTypeDev, Host: "localhost", Port: 3306, Username: "root", Charset: "utf8mb4"},
				{ID: "prod", Name: "生产环境", Type: config.EnvTypeProd, Host: "localhost", Port: 3306, Username: "root", Charset: "utf8mb4"},
			},
			IgnoreRules: config.IgnoreConfig{
				IgnoreComments:  true,
				IgnoreCharset:   true,
				IgnoreCollation: true,
			},
		}
		mw.store.AddProject(*project)
		mw.store.SetActiveProject(project.ID)
	}

	// 加载环境配置
	for _, env := range project.Environments {
		envCopy := env // 避免循环变量问题
		if env.Type == config.EnvTypeDev {
			mw.sourceEnvPanel.SetEnvironment(&envCopy)
		} else if env.Type == config.EnvTypeProd {
			mw.targetEnvPanel.SetEnvironment(&envCopy)
		}
	}

	// 加载忽略选项（检查 nil 避免初始化时崩溃）
	if mw.ignoreComments != nil {
		mw.ignoreComments.SetChecked(project.IgnoreRules.IgnoreComments)
	}
	if mw.ignoreCharset != nil {
		mw.ignoreCharset.SetChecked(project.IgnoreRules.IgnoreCharset)
	}
	if mw.ignoreCollation != nil {
		mw.ignoreCollation.SetChecked(project.IgnoreRules.IgnoreCollation)
	}
}

// saveConfig 保存配置
func (mw *MainWindow) saveConfig() {
	project := mw.store.GetActiveProject()
	if project == nil {
		project = &config.Project{
			ID:           "default",
			Name:         "默认项目",
			Environments: []config.Environment{},
		}
	}

	// 更新环境配置
	sourceEnv := mw.sourceEnvPanel.GetEnvironment()
	targetEnv := mw.targetEnvPanel.GetEnvironment()

	// 重建环境列表
	newEnvs := []config.Environment{}
	if sourceEnv != nil {
		newEnvs = append(newEnvs, *sourceEnv)
	}
	if targetEnv != nil {
		newEnvs = append(newEnvs, *targetEnv)
	}
	project.Environments = newEnvs

	// 更新忽略规则（检查 nil 避免初始化时崩溃）
	if mw.ignoreComments != nil {
		project.IgnoreRules.IgnoreComments = mw.ignoreComments.Checked
	}
	if mw.ignoreCharset != nil {
		project.IgnoreRules.IgnoreCharset = mw.ignoreCharset.Checked
	}
	if mw.ignoreCollation != nil {
		project.IgnoreRules.IgnoreCollation = mw.ignoreCollation.Checked
	}

	// 保存
	if err := mw.store.UpdateProject(*project); err != nil {
		zap.S().Errorf("保存配置失败: %v", err)
	}
}

// updateIgnoreRules 更新忽略规则并保存
func (mw *MainWindow) updateIgnoreRules() {
	mw.saveConfig()
}
