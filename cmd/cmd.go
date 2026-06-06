package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use: "v2node",
}

func Run() {
	err := command.Execute()
	if err != nil {
		log.WithField("err", err).Error("Execute command failed")
	}
	// 不调用 os.Exit(1)，让 serverHandle 中的 defer 正常清理资源
	// 进程退出码由 systemd/容器编排根据服务状态决定
}
