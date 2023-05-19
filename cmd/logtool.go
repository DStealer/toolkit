package cmd

import (
	"github.com/bmatcuk/doublestar"
	"github.com/robfig/cron"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var (
	logCmd = &cobra.Command{
		Use:   "log subcommand [args]",
		Short: "log 管理工具",
	}
	logDryRun bool
)

func init() {
	logCmd.PersistentFlags().BoolVar(&logDryRun, "dryRun", logDryRun, "测试运行")
	tuncCmd := &cobra.Command{
		Use:   "truncate [path expression]",
		Short: "截断符合条件的文件数据",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if logDryRun {
				log.Info("*********执行截断操作(测试)************")
			} else {
				log.Info("*********执行截断操作************")
			}
			truncateLog(args[0], logDryRun)
			log.Info("**********结束运行***********")
		},
	}
	logCmd.AddCommand(tuncCmd)

	delCmd := &cobra.Command{
		Use:   "delete [path expression]",
		Short: "删除符合条件的文件数据",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if logDryRun {
				log.Info("*********执行删除操作(测试)************")
			} else {
				log.Info("*********执行删除操作************")
			}
			duration, err := cmd.Flags().GetDuration("change")
			cobra.CheckErr(err)
			cronExp, err := cmd.Flags().GetString("cronExp")
			cobra.CheckErr(err)
			if cronExp != "" {
				c := cron.New()
				err := c.AddFunc(cronExp, func() {
					deleteLog(args[0], duration, logDryRun)
				})
				cobra.CheckErr(err)
				defer c.Stop()
				c.Start()
				entries := c.Entries()
				if len(entries) > 0 {
					log.Info("定时任务已启动,首次执行时间是:", entries[0].Next.Format("2006-01-02 15:04:05"))
				} else {
					log.Info("定时任务已启动")
				}
				select {}
			} else {
				deleteLog(args[0], duration, logDryRun)
			}

			deleteLog(args[0], duration, logDryRun)
			log.Info("**********结束运行***********")
		},
	}
	delCmd.Flags().Duration("change", time.Duration(0), "change duration bigger than")
	delCmd.Flags().String("cronExp", "", "cronExp 表达式,如果填写则在后台定时运行 Second | Minute | Hour | Dom | Month | DowOptional | Descriptor")
	logCmd.AddCommand(delCmd)
	cron.New()
}

// 删除日志文件
func deleteLog(pathExp string, duration time.Duration, dryRun bool) {
	paths, err := doublestar.Glob(pathExp)
	cobra.CheckErr(err)
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			log.Infof("[%s]处理错误:%s,忽略\n", path, err.Error())
			continue
		}
		if stat.IsDir() {
			log.Infof("忽略文件夹[%s]\n", path)
			continue
		}
		if duration != time.Duration(0) && stat.ModTime().Add(duration).After(time.Now()) {
			log.Infof("文件:[%s]变更时间:%s,忽略", path, stat.ModTime().Format("2006-01-02 15:04:05"))
			continue
		}
		log.Infof("删除文件:[%s] %v\n", path, stat.ModTime().Format("2006-01-02 15:04:05"))
		if !dryRun {
			err := os.Remove(path)
			if err != nil {
				log.Warnf("删除文件:[%s]错误,%s\n", path, err.Error())
			}
		}
	}
}

// 截断日志文件
func truncateLog(pathExp string, dryRun bool) {
	paths, err := doublestar.Glob(pathExp)
	cobra.CheckErr(err)
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			log.Infof("[%s]处理错误:%s,忽略执行\n", path, err.Error())
			continue
		}
		if stat.IsDir() {
			log.Warnf("忽略文件夹[%s]\n", path)
			continue
		}
		log.Infof("截断文件:[%s]\n", path)
		if !dryRun {
			func(path string) {
				err := os.Truncate(path, 0)
				if err != nil {
					log.Warnf("截断文件:[%s]错误,%s\n", path, err.Error())
				}
			}(path)
		}
	}
}
