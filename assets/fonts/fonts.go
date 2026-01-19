package fonts

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed LXGWWenKai-Regular.ttf
var lxgwWenKai []byte

// LXGWWenKai 返回霞鹜文楷字体资源
func LXGWWenKai() fyne.Resource {
	return fyne.NewStaticResource("LXGWWenKai-Regular.ttf", lxgwWenKai)
}
