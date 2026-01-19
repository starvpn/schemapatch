package main

import (
	"os"

	"github.com/starvpn/schemapatch/internal/gui"
	"go.uber.org/zap"
)

func main() {
	// 初始化日志
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	defer logger.Sync()

	zap.ReplaceGlobals(logger)
	zap.S().Info("SchemaPatch 启动中...")

	// 启动GUI应用
	app := gui.NewApp()
	app.Run()
}
