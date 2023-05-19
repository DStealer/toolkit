package cmd

import (
	"github.com/spf13/cobra"
)

var (
	mysqlAddr     = "127.0.0.1:3306"
	mysqlUser     = ""
	mysqlPassword = ""
	mysqlDatabase = ""

	mysqlCmd = &cobra.Command{
		Use:   "mysql subcommand [args]",
		Short: "mysql运维管理工具",
	}
)

func init() {
	mysqlCmd.PersistentFlags().StringVar(&mysqlAddr, "addr", server, "服务地址数据库地址,ip:port或unix socket")
	mysqlCmd.PersistentFlags().StringVar(&mysqlUser, "user", server, "用户名")
	mysqlCmd.PersistentFlags().StringVar(&mysqlPassword, "password", server, "密码")
	mysqlCmd.PersistentFlags().StringVar(&mysqlDatabase, "database", server, "数据库名称")

	dumpCmd := &cobra.Command{
		Use:   "dump [args] table",
		Short: "mysql数据导出工具",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

		},
	}
	dumpCmd.Flags().String("where", "", "查询条件")
	mysqlCmd.AddCommand(dumpCmd)
}
