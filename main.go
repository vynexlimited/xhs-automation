package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

func main() {
	startupConfig, err := configs.ParseStartupFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(configs.StartupHelpText())
			os.Exit(0)
		}
		logrus.Fatalf("failed to parse startup flags: %v", err)
	}

	if len(startupConfig.BinPath) == 0 {
		startupConfig.BinPath = os.Getenv("ROD_BROWSER_BIN")
	}

	configs.InitHeadless(startupConfig.Headless)
	configs.SetBinPath(startupConfig.BinPath)
	configs.SetCdpURL(startupConfig.CDPURL)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(startupConfig.Port); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
