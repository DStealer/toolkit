package cmd

import (
	"fmt"
	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"strings"
)

var (
	mysqlAddr     = "127.0.0.1:3306"
	mysqlUsername = ""
	mysqlPassword = ""
	mysqlDatabase = ""

	mysqlCmd = &cobra.Command{
		Use:   "mysql subcommand [args]",
		Short: "mysql运维管理工具",
	}
)

func init() {
	mysqlCmd.PersistentFlags().StringVar(&mysqlAddr, "addr", redisServer, "服务地址数据库地址,ip:port或unix socket")
	mysqlCmd.PersistentFlags().StringVar(&mysqlUsername, "username", redisServer, "用户名")
	mysqlCmd.PersistentFlags().StringVar(&mysqlPassword, "password", redisServer, "密码")
	mysqlCmd.PersistentFlags().StringVar(&mysqlDatabase, "database", redisServer, "数据库名称")

	dumpCmd := &cobra.Command{
		Use:   "dump [args] table",
		Short: "mysql数据导出工具",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, err := client.Connect(mysqlAddr, mysqlUsername, mysqlPassword, mysqlDatabase)
			cobra.CheckErr(err)
			err = conn.Ping()
			if err != nil {
				log.Fatalln("数据库连接失败", err)
			}
			table := args[0]
			where, err := cmd.Flags().GetString("where")
			cobra.CheckErr(err)
			if where == "" {
				where = "1=1"
			}
			defer conn.Close()
			var result mysql.Result
			defer result.Close()

			err = conn.ExecuteSelectStreaming(fmt.Sprintf("SELECT * FROM `%s` WHERE %s ;", table, where), &result, func(row []mysql.FieldValue) error {
				names := make([]string, len(result.Fields))
				values := make([]string, len(result.Fields))
				for index, val := range row {
					if val.Type == mysql.FieldValueTypeString {
						values[index] = fmt.Sprintf("'%s'", string(val.AsString()))
					} else if val.Type == mysql.FieldValueTypeNull {
						values[index] = "NULL"
					} else {
						values[index] = fmt.Sprintf("%v", val.Value())
					}
					names[index] = fmt.Sprintf("`%s`", string(result.Fields[index].Name))
				}
				fmt.Printf("INSERT INTO `%s` (%s) VALUES (%s);\n", table, strings.Join(names, ","), strings.Join(values, ","))
				return nil
			}, func(result *mysql.Result) error {
				return nil
			})
			cobra.CheckErr(err)
		},
	}
	dumpCmd.Flags().String("where", "", "查询条件")
	mysqlCmd.AddCommand(dumpCmd)
}
