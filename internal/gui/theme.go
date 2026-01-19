package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/schemapatch/schemapatch/assets/fonts"
)

// SchemaPatchTheme 自定义主题
type SchemaPatchTheme struct {
	fyne.Theme
	font fyne.Resource
}

// NewSchemaPatchTheme 创建自定义主题
func NewSchemaPatchTheme() *SchemaPatchTheme {
	return &SchemaPatchTheme{
		Theme: theme.DarkTheme(),
		font:  fonts.LXGWWenKai(),
	}
}

// Font 返回字体
func (t *SchemaPatchTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.font
}

// Color 自定义颜色
func (t *SchemaPatchTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 30, G: 30, B: 46, A: 255} // #1E1E2E
	case theme.ColorNameButton:
		return color.NRGBA{R: 49, G: 50, B: 68, A: 255} // #313244
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 108, G: 112, B: 134, A: 255} // #6C7086
	case theme.ColorNameForeground:
		return color.NRGBA{R: 205, G: 214, B: 244, A: 255} // #CDD6F4
	case theme.ColorNameHover:
		return color.NRGBA{R: 69, G: 71, B: 90, A: 255} // #45475A
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 49, G: 50, B: 68, A: 255} // #313244
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 137, G: 180, B: 250, A: 255} // #89B4FA
	case theme.ColorNameSelection:
		return color.NRGBA{R: 69, G: 71, B: 90, A: 255} // #45475A
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 166, G: 227, B: 161, A: 255} // #A6E3A1
	case theme.ColorNameWarning:
		return color.NRGBA{R: 249, G: 226, B: 175, A: 255} // #F9E2AF
	case theme.ColorNameError:
		return color.NRGBA{R: 243, G: 139, B: 168, A: 255} // #F38BA8
	}
	return t.Theme.Color(name, variant)
}

// Icon 图标
func (t *SchemaPatchTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.Theme.Icon(name)
}

// Size 尺寸
func (t *SchemaPatchTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 4
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 16
	}
	return t.Theme.Size(name)
}

// 自定义颜色常量
var (
	// 状态颜色
	ColorDanger  = color.NRGBA{R: 243, G: 139, B: 168, A: 255} // #F38BA8
	ColorWarning = color.NRGBA{R: 249, G: 226, B: 175, A: 255} // #F9E2AF
	ColorSuccess = color.NRGBA{R: 166, G: 227, B: 161, A: 255} // #A6E3A1
	ColorInfo    = color.NRGBA{R: 137, G: 180, B: 250, A: 255} // #89B4FA

	// 表面颜色
	ColorSurface0 = color.NRGBA{R: 49, G: 50, B: 68, A: 255}  // #313244
	ColorSurface1 = color.NRGBA{R: 69, G: 71, B: 90, A: 255}  // #45475A
	ColorSurface2 = color.NRGBA{R: 88, G: 91, B: 112, A: 255} // #585B70

	// 文字颜色
	ColorText     = color.NRGBA{R: 205, G: 214, B: 244, A: 255} // #CDD6F4
	ColorSubtext0 = color.NRGBA{R: 166, G: 173, B: 200, A: 255} // #A6ADC8
	ColorSubtext1 = color.NRGBA{R: 147, G: 153, B: 178, A: 255} // #939AB2

	// 强调色
	ColorBlue   = color.NRGBA{R: 137, G: 180, B: 250, A: 255} // #89B4FA
	ColorGreen  = color.NRGBA{R: 166, G: 227, B: 161, A: 255} // #A6E3A1
	ColorYellow = color.NRGBA{R: 249, G: 226, B: 175, A: 255} // #F9E2AF
	ColorRed    = color.NRGBA{R: 243, G: 139, B: 168, A: 255} // #F38BA8
	ColorPink   = color.NRGBA{R: 245, G: 194, B: 231, A: 255} // #F5C2E7
	ColorMauve  = color.NRGBA{R: 203, G: 166, B: 247, A: 255} // #CBA6F7
	ColorTeal   = color.NRGBA{R: 148, G: 226, B: 213, A: 255} // #94E2D5
	ColorPeach  = color.NRGBA{R: 250, G: 179, B: 135, A: 255} // #FAB387
)
