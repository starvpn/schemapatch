package gui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/starvpn/schemapatch/internal/config"
	"github.com/starvpn/schemapatch/internal/extractor"
)

// EnvPanel 环境配置面板
type EnvPanel struct {
	title       string
	accentColor color.Color
	envType     config.EnvironmentType

	// 输入字段
	hostEntry     *widget.Entry
	portEntry     *widget.Entry
	usernameEntry *widget.Entry
	passwordEntry *widget.Entry
	databaseEntry *widget.Entry

	// 状态
	statusLabel *widget.Label
	testBtn     *widget.Button

	// 容器
	container *fyne.Container

	// 变更回调
	onChanged func()
}

// NewEnvPanel 创建环境配置面板
func NewEnvPanel(title string, accentColor color.Color, envType config.EnvironmentType) *EnvPanel {
	ep := &EnvPanel{
		title:       title,
		accentColor: accentColor,
		envType:     envType,
	}

	ep.build()
	return ep
}

// SetOnChanged 设置变更回调
func (ep *EnvPanel) SetOnChanged(callback func()) {
	ep.onChanged = callback
}

// notifyChanged 通知配置变更
func (ep *EnvPanel) notifyChanged() {
	if ep.onChanged != nil {
		ep.onChanged()
	}
}

// build 构建面板
func (ep *EnvPanel) build() {
	// 标题
	titleLabel := widget.NewLabelWithStyle(ep.title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// 颜色指示条
	colorBar := canvas.NewRectangle(ep.accentColor)
	colorBar.SetMinSize(fyne.NewSize(0, 4))

	// 输入字段变更处理
	onTextChanged := func(s string) {
		ep.notifyChanged()
	}

	// 输入字段
	ep.hostEntry = widget.NewEntry()
	ep.hostEntry.SetPlaceHolder("主机地址")
	ep.hostEntry.SetText("localhost")
	ep.hostEntry.OnChanged = onTextChanged

	ep.portEntry = widget.NewEntry()
	ep.portEntry.SetPlaceHolder("端口")
	ep.portEntry.SetText("3306")
	ep.portEntry.OnChanged = onTextChanged

	ep.usernameEntry = widget.NewEntry()
	ep.usernameEntry.SetPlaceHolder("用户名")
	ep.usernameEntry.SetText("root")
	ep.usernameEntry.OnChanged = onTextChanged

	ep.passwordEntry = widget.NewPasswordEntry()
	ep.passwordEntry.SetPlaceHolder("密码")
	ep.passwordEntry.OnChanged = onTextChanged

	ep.databaseEntry = widget.NewEntry()
	ep.databaseEntry.SetPlaceHolder("数据库名")
	ep.databaseEntry.OnChanged = onTextChanged

	// 表单布局
	form := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("主机:"),
			ep.hostEntry,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("端口:"),
			ep.portEntry,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("用户名:"),
			ep.usernameEntry,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("密码:"),
			ep.passwordEntry,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("数据库:"),
			ep.databaseEntry,
		),
	)

	// 测试连接按钮和状态
	ep.statusLabel = widget.NewLabel("")
	ep.testBtn = widget.NewButtonWithIcon("测试连接", theme.ConfirmIcon(), ep.onTestConnection)

	buttonRow := container.NewHBox(
		ep.testBtn,
		ep.statusLabel,
	)

	// 组装面板
	ep.container = container.NewVBox(
		colorBar,
		titleLabel,
		widget.NewSeparator(),
		form,
		buttonRow,
	)
}

// Container 获取容器
func (ep *EnvPanel) Container() *fyne.Container {
	return ep.container
}

// GetEnvironment 获取环境配置
func (ep *EnvPanel) GetEnvironment() *config.Environment {
	port, _ := strconv.Atoi(ep.portEntry.Text)
	if port == 0 {
		port = 3306
	}

	return &config.Environment{
		ID:       string(ep.envType),
		Name:     ep.title,
		Type:     ep.envType,
		Host:     ep.hostEntry.Text,
		Port:     port,
		Username: ep.usernameEntry.Text,
		Password: ep.passwordEntry.Text,
		Database: ep.databaseEntry.Text,
		Charset:  "utf8mb4",
	}
}

// SetEnvironment 设置环境配置
func (ep *EnvPanel) SetEnvironment(env *config.Environment) {
	if env == nil {
		return
	}

	ep.hostEntry.SetText(env.Host)
	ep.portEntry.SetText(fmt.Sprintf("%d", env.Port))
	ep.usernameEntry.SetText(env.Username)
	ep.passwordEntry.SetText(env.Password)
	ep.databaseEntry.SetText(env.Database)
}

// onTestConnection 测试连接
func (ep *EnvPanel) onTestConnection() {
	ep.statusLabel.SetText("测试中...")
	ep.testBtn.Disable()

	go func() {
		env := ep.GetEnvironment()

		// 创建提取器测试连接
		ext, err := extractor.NewMySQLExtractor(env)
		if err != nil {
			ep.statusLabel.SetText("❌ 失败")
			ep.testBtn.Enable()
			return
		}
		defer ext.Close()

		ctx := context.Background()
		if err := ext.TestConnection(ctx); err != nil {
			ep.statusLabel.SetText("❌ " + err.Error())
		} else {
			ep.statusLabel.SetText("✅ 连接成功")
		}

		ep.testBtn.Enable()
	}()
}

// Validate 验证输入
func (ep *EnvPanel) Validate() error {
	if ep.hostEntry.Text == "" {
		return fmt.Errorf("请输入主机地址")
	}
	if ep.databaseEntry.Text == "" {
		return fmt.Errorf("请输入数据库名")
	}
	return nil
}
