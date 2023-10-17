package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/go-mysql-org/go-mysql/client"
	_ "github.com/go-mysql-org/go-mysql/driver"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/pingcap/tidb/parser"
	_ "github.com/pingcap/tidb/parser/test_driver"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"os"
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
	mysqlCmd.PersistentFlags().StringVar(&mysqlAddr, "addr", mysqlAddr, "服务地址数据库地址,ip:port或unix socket")
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
			fmt.Printf("#数据库:%s 表:%s 操作时间:%s\n", mysqlDatabase, table, time.Now().Format("2006-01-02 15:04:05"))
			defer conn.Close()
			//保证数据一致性
			defer conn.Rollback()

			cobra.CheckErr(err)
			_, err = conn.Execute("SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;")
			cobra.CheckErr(err)
			_, err = conn.Execute("START TRANSACTION /*!40100 WITH CONSISTENT SNAPSHOT */;")
			cobra.CheckErr(err)
			var result mysql.Result
			defer result.Close()
			var recordsNo int
			err = conn.ExecuteSelectStreaming(fmt.Sprintf("SELECT /*!40001 SQL_NO_CACHE */ * FROM `%s` WHERE %s ;", table, where), &result, func(row []mysql.FieldValue) error {
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

	cleansingCmd := &cobra.Command{
		Use:   "cleansing subcommand [args]",
		Short: "mysql数据清洗工具",
	}

	cleansingUpdateCmd := &cobra.Command{
		Use:   "update configfile",
		Short: "mysql数据清洗工具-更新",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, err := client.Connect(mysqlAddr, mysqlUsername, mysqlPassword, mysqlDatabase)
			defer conn.Close()
			cobra.CheckErr(err)
			err = conn.Ping()
			if err != nil {
				log.Fatalln("数据库连接失败", err)
			}
			file, err := os.ReadFile(args[0])
			cobra.CheckErr(err)
			var mysqlCleansingConfig MysqlCleansingConfig
			if strings.HasSuffix(args[0], ".yaml") {
				err = yaml.Unmarshal(file, &mysqlCleansingConfig)
				cobra.CheckErr(err)
			} else if strings.HasSuffix(args[0], ".json") {
				err = json.Unmarshal(file, &mysqlCleansingConfig)
				cobra.CheckErr(err)
			} else {
				cobra.CheckErr("不支持的配置文件格式")
			}
			log.Infof("开始校验文件")
			mysqlCleansingConfig.validate()
			log.Infof("开始执行程序")
			for index, item := range mysqlCleansingConfig.Items {
				log.Infof("开始处理%d/%d条目%s.%s", index+1, len(mysqlCleansingConfig.Items), item.Schema, item.Table)
				log.Infof("语句:%s", item.UpdateSql)
				result, err := conn.Execute(fmt.Sprintf("SHOW KEYS FROM `%s`.%s WHERE Key_name = 'PRIMARY' ", item.Schema, item.Table))
				cobra.CheckErr(err)
				if result.RowNumber() != 1 {
					cobra.CheckErr("查询主键错误")
				}
				keyName, err := result.GetStringByName(0, "Column_name")
				cobra.CheckErr(err)
				result.Close()
				result, err = conn.Execute(fmt.Sprintf("select min(%s) as Lid, max(%s) as Hid from `%s`.%s", keyName, keyName, item.Schema, item.Table))
				cobra.CheckErr(err)
				if result.RowNumber() != 1 {
					cobra.CheckErr("查询主键边界错误")
				}
				lowId, err := result.GetIntByName(0, "Lid")
				cobra.CheckErr(err)
				highId, err := result.GetIntByName(0, "Hid")
				cobra.CheckErr(err)
				result.Close()
				log.Infof("查询当前数据上下边界:%v-%v", lowId, highId)
				if item.StartId != 0 || item.EndId != 0 {
					lowId = If(item.StartId == 0, lowId, item.StartId).(int64)
					highId = If(item.EndId == 0, highId, item.EndId).(int64)
					log.Infof("调整数据边界:%v-%v", lowId, highId)
				}
				generator, err := NewPairGenerator(lowId, highId, mysqlCleansingConfig.BatchSize)
				cobra.CheckErr(err)
				var totalAffectedRows uint64 = 0
				for {
					next, left, right := generator.NextBoundary()
					if !next {
						break
					}
					result, err = conn.Execute(item.UpdateSql, left, right)
					cobra.CheckErr(err)
					log.Infof("执行:%v-%v,记录:%v条", left, right, result.AffectedRows)
					totalAffectedRows = totalAffectedRows + result.AffectedRows
					result.Close()
				}
				log.Infof("结束处理%d/%d条目%s.%s 共处理%d条", index+1, len(mysqlCleansingConfig.Items), item.Schema, item.Table, totalAffectedRows)
			}
			log.Infof("结束执行")
		},
	}
	cleansingCmd.AddCommand(cleansingUpdateCmd)

	cleansingValidateCmd := &cobra.Command{
		Use:   "validate configfile",
		Short: "mysql数据清洗工具-校验",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, err := client.Connect(mysqlAddr, mysqlUsername, mysqlPassword, mysqlDatabase)
			defer conn.Close()
			cobra.CheckErr(err)
			err = conn.Ping()
			if err != nil {
				log.Fatalln("数据库连接失败", err)
			}
			file, err := os.ReadFile(args[0])
			cobra.CheckErr(err)
			var mysqlCleansingConfig MysqlCleansingConfig
			if strings.HasSuffix(args[0], ".yaml") {
				err = yaml.Unmarshal(file, &mysqlCleansingConfig)
				cobra.CheckErr(err)
			} else if strings.HasSuffix(args[0], ".json") {
				err = json.Unmarshal(file, &mysqlCleansingConfig)
				cobra.CheckErr(err)
			} else {
				cobra.CheckErr("不支持的配置文件格式")
			}
			log.Infof("开始校验文件")
			mysqlCleansingConfig.validate()
			log.Infof("开始执行程序")
			for index, item := range mysqlCleansingConfig.Items {
				log.Infof("开始处理%d/%d条目%s.%s", index+1, len(mysqlCleansingConfig.Items), item.Schema, item.Table)
				log.Infof("语句:%s", item.ValidateSql)
				result, err := conn.Execute(fmt.Sprintf("SHOW KEYS FROM `%s`.%s WHERE Key_name = 'PRIMARY' ", item.Schema, item.Table))
				cobra.CheckErr(err)
				if result.RowNumber() != 1 {
					cobra.CheckErr("查询主键错误")
				}
				keyName, err := result.GetStringByName(0, "Column_name")
				cobra.CheckErr(err)
				result.Close()
				result, err = conn.Execute(fmt.Sprintf("select min(%s) as Lid, max(%s) as Hid from `%s`.%s", keyName, keyName, item.Schema, item.Table))
				cobra.CheckErr(err)
				if result.RowNumber() != 1 {
					cobra.CheckErr("查询主键边界错误")
				}
				lowId, err := result.GetIntByName(0, "Lid")
				cobra.CheckErr(err)
				highId, err := result.GetIntByName(0, "Hid")
				cobra.CheckErr(err)
				result.Close()
				log.Infof("查询当前数据上下边界:%v-%v", lowId, highId)
				if item.StartId != 0 || item.EndId != 0 {
					lowId = If(item.StartId == 0, lowId, item.StartId).(int64)
					highId = If(item.EndId == 0, highId, item.EndId).(int64)
					log.Infof("调整数据边界:%v-%v", lowId, highId)
				}
				generator, err := NewPairGenerator(lowId, highId, mysqlCleansingConfig.BatchSize)
				cobra.CheckErr(err)
				var totalAffectedRows = 0
				for {
					next, left, right := generator.NextBoundary()
					if !next {
						break
					}
					result, err = conn.Execute(item.ValidateSql, left, right)
					cobra.CheckErr(err)
					log.Infof("执行:%v-%v,记录:%v条", left, right, result.RowNumber())
					totalAffectedRows = totalAffectedRows + result.RowNumber()

					for _, row := range result.Values {
						values := make([]string, len(result.Fields))
						for index, val := range row {
							if val.Type == mysql.FieldValueTypeString {
								values[index] = fmt.Sprintf("'%s'", string(val.AsString()))
							} else if val.Type == mysql.FieldValueTypeNull {
								values[index] = "NULL"
							} else {
								values[index] = fmt.Sprintf("%v", val.Value())
							}
						}
						fmt.Println("record:", values)
					}
					result.Close()
				}
				log.Infof("结束处理%d/%d条目%s.%s 共处理%d条\n", index+1, len(mysqlCleansingConfig.Items), item.Schema, item.Table, totalAffectedRows)
			}
			log.Infof("结束执行")
		},
	}
	cleansingCmd.AddCommand(cleansingValidateCmd)

	mysqlCmd.AddCommand(cleansingCmd)
}

type MysqlCleansingConfig struct {
	BatchSize int64                `yaml:"batchSize" json:"batchSize"`
	Items     []MysqlCleansingItem `yaml:"items" json:"items"`
}

func (c MysqlCleansingConfig) validate() {
	if c.BatchSize <= 0 {
		cobra.CheckErr("batchSize配置错误")
	}
	if len(c.Items) == 0 {
		cobra.CheckErr("请至少配置一条规则")
	}
	for _, item := range c.Items {
		item.validate()
	}
}

type MysqlCleansingItem struct {
	Schema      string `yaml:"schema" json:"schema"`
	Table       string `yaml:"table" json:"table"`
	UpdateSql   string `yaml:"updateSql" json:"updateSql"`
	ValidateSql string `yaml:"validateSql" json:"validateSql"`
	StartId     int64  `yaml:"startId" json:"startId"`
	EndId       int64  `yaml:"endId" json:"endId"`
}

func (c MysqlCleansingItem) validate() {
	if c.Schema == "" {
		cobra.CheckErr("schema配置错误")
	}
	if c.Table == "" {
		cobra.CheckErr("table配置错误")
	}
	if c.StartId < 0 {
		cobra.CheckErr("StartId配置错误")
	}
	if c.EndId < 0 {
		cobra.CheckErr("EndId配置错误")
	}
	if c.UpdateSql != "" {
		p := parser.New()
		_, _, err := p.Parse(c.UpdateSql, "", "")
		cobra.CheckErr(err)
	}
	if c.ValidateSql != "" {
		p := parser.New()
		_, _, err := p.Parse(c.ValidateSql, "", "")
		cobra.CheckErr(err)
	}
}
