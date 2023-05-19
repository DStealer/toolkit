package cmd

import (
	"fmt"
	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"strings"
	"time"
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
	mysqlCmd.PersistentFlags().StringVar(&mysqlUsername, "username", mysqlUsername, "用户名")
	mysqlCmd.PersistentFlags().StringVar(&mysqlPassword, "password", mysqlPassword, "密码")
	mysqlCmd.PersistentFlags().StringVar(&mysqlDatabase, "database", mysqlDatabase, "数据库名称")

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
			fmt.Printf("#数据库:%s 表:%s 时间:%s\n", mysqlDatabase, table, time.Now().Format("2006-01-02 15:04:05"))
			defer conn.Close()
			//保证数据一致性
			defer conn.Rollback()
			err = conn.Begin()
			cobra.CheckErr(err)

			var result mysql.Result
			defer result.Close()
			var recordsNo int
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
				recordsNo += 1
				return nil
			}, func(result *mysql.Result) error {
				return nil
			})
			fmt.Printf("#总计数目:%d条\n", recordsNo)
			cobra.CheckErr(err)
		},
	}
	dumpCmd.Flags().String("where", "", "查询条件")
	mysqlCmd.AddCommand(dumpCmd)
}
