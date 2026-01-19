package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/schemapatch/schemapatch/internal/config"
	"go.uber.org/zap"
)

// App 应用主体
type App struct {
	fyneApp    fyne.App
	mainWindow *MainWindow
	store      *config.Store
}

// NewApp 创建应用
func NewApp() *App {
	fyneApp := app.NewWithID("com.schemapatch.app")
	fyneApp.Settings().SetTheme(NewSchemaPatchTheme())

	// 初始化配置存储
	store, err := config.NewStore()
	if err != nil {
		zap.S().Warnf("加载配置失败，使用默认配置: %v", err)
	}

	return &App{
		fyneApp: fyneApp,
		store:   store,
	}
}

// Run 运行应用
func (a *App) Run() {
	// 创建主窗口
	a.mainWindow = NewMainWindow(a.fyneApp, a.store)
	a.mainWindow.Show()

	// 运行应用
	a.fyneApp.Run()
}

// Quit 退出应用
func (a *App) Quit() {
	a.fyneApp.Quit()
}
