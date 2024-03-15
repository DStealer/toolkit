package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/bgentry/speakeasy"
	"github.com/docker/go-units"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var (
	osCmd = &cobra.Command{
		Use:   "os subcommand [args]",
		Short: "os运维管理工具",
	}
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	resourceCmd := &cobra.Command{
		Use:   "resource [args]",
		Short: "系统使用率优化工具",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {

			ctx, cancelFunc := context.WithCancel(context.Background())
			cpuPercent, err := cmd.Flags().GetFloat64("cpuPercent")
			cobra.CheckErr(err)
			cpuTolerant, err := cmd.Flags().GetFloat64("cpuTolerant")
			cobra.CheckErr(err)
			if cpuPercent < 0.0 || cpuPercent > 1.0 {
				cobra.CheckErr("cpuPercent must between 0.0 and 1.0")
			}
			if cpuTolerant < -0.5 || cpuTolerant > 0.5 {
				cobra.CheckErr("cpuTolerant must between -0.5 and 0.5")
			}
			memPercent, err := cmd.Flags().GetFloat64("memPercent")
			cobra.CheckErr(err)
			memTolerant, err := cmd.Flags().GetFloat64("memTolerant")
			cobra.CheckErr(err)

			if memPercent < 0.0 || memPercent > 1.0 {
				cobra.CheckErr("memPercent must between 0.0 and 1.0")
			}
			if memTolerant < 0.0 || memTolerant > 0.5 {
				cobra.CheckErr("memTolerant must between 0 and 0.5")
			}
			durationMin, err := cmd.Flags().GetInt64("durationMin")
			cobra.CheckErr(err)
			log.Info("cpuPercent=", cpuPercent, "cpuTolerant=", cpuTolerant)
			keepCpu(cpuPercent, cpuTolerant, ctx)
			log.Info("memPercent=", cpuPercent, "memTolerant=", cpuTolerant)
			keepMem(memPercent, memTolerant, ctx)

			if durationMin >= 0 {
				ticker := time.NewTicker(time.Duration(durationMin) * time.Minute)
				select {
				case <-ctx.Done():
					log.Errorf("错误:%v\n", ctx.Err())
				case <-ticker.C:
					cancelFunc()
					log.Info("正常退出")
				}
			} else {
				select {}
			}
		},
	}
	resourceCmd.Flags().Float64("cpuPercent", 0, "cpu目标使用率")
	resourceCmd.MarkFlagRequired("cpuPercent")
	resourceCmd.Flags().Float64("cpuTolerant", 0.1, "cpu目标容忍使用率")

	resourceCmd.Flags().Float64("memPercent", 0, "mem目标使用率")
	resourceCmd.MarkFlagRequired("memPercent")
	resourceCmd.Flags().Float64("memTolerant", 0.1, "mem目标容忍使用率")

	resourceCmd.Flags().Int64("durationMin", -1, "持续时长(分钟),-1表示永久")

	osCmd.AddCommand(resourceCmd)

	batchCmd := &cobra.Command{
		Use:   "batch [args] command-file",
		Short: "批量执行命令工具,文件中每一行为一条命令",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********开始执行*******")
			startLine, err := cmd.Flags().GetInt("start-line")
			cobra.CheckErr(err)
			fastFail, err := cmd.Flags().GetBool("fast-fail")
			cobra.CheckErr(err)
			file, err := os.OpenFile(args[0], os.O_RDONLY, 0644)
			cobra.CheckErr(err)
			defer file.Close()
			br := bufio.NewReader(file)
			lineCounter := 0
			successCounter := 0
			failedCounter := 0
			ignoreCounter := 0
			for {
				line, err := br.ReadString('\n')
				if err != nil || io.EOF == err {
					break
				}
				lineCounter = lineCounter + 1
				if startLine > lineCounter {
					ignoreCounter = ignoreCounter + 1
					log.Infof("忽略第%v条命令:%v", lineCounter, line)
					continue
				}
				log.Infof("执行第%v条命令:%v", lineCounter, line)
				cmd := exec.Command("/bin/bash", "-c", line)
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err = cmd.Run()
				if err != nil {
					failedCounter = failedCounter + 1
					if stderr.Len() > 0 {
						log.Warnf("执行第%v条命令失败,输出为:\n%s", lineCounter, stderr.String())
					} else {
						log.Warnf("执行第%v条命令失败", lineCounter)
					}
					if fastFail {
						break
					}
				} else {
					successCounter = successCounter + 1
					if stdout.Len() > 0 {
						log.Infof("执行第%v条命令成功,输出为:\n%s", lineCounter, stdout.String())
					} else {
						log.Infof("执行第%v条命令成功", lineCounter)
					}
				}
			}
			log.Info("**********执行结束***********")
			log.Infof(
				"总计执行命令:%v条,成功:%v条,失败:%v条,忽略:%v条", lineCounter, successCounter, failedCounter,
				ignoreCounter)
		},
	}
	batchCmd.Flags().Int("start-line", 0, "命令行开始位置")
	batchCmd.Flags().Bool("fast-fail", true, "命令失败是否继续执行")
	osCmd.AddCommand(batchCmd)

	sshBatchCmd := &cobra.Command{
		Use:   "ssh-batch [args] command-file",
		Short: "批量执行命令工具,文件中每一行为一条命令",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("**********开始执行*******")

			addr, err := cmd.Flags().GetString("addr")
			cobra.CheckErr(err)
			user, err := cmd.Flags().GetString("user")
			cobra.CheckErr(err)
			log.Infof("尝试连接[%s@%s]", user, addr)
			password, err := speakeasy.Ask("请输入密码:")
			cobra.CheckErr(err)
			if password == "" {
				fmt.Println("选择退出操作")
				os.Exit(0)
			}
			config := &ssh.ClientConfig{
				User: user,
				Auth: []ssh.AuthMethod{
					ssh.Password(password),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         60 * time.Second,
			}
			client, err := ssh.Dial("tcp", addr, config)
			cobra.CheckErr(err)
			log.Infof("连接[%s@%s]成功", user, addr)
			defer client.Close()
			startLine, err := cmd.Flags().GetInt("start-line")
			cobra.CheckErr(err)
			fastFail, err := cmd.Flags().GetBool("fast-fail")
			cobra.CheckErr(err)
			file, err := os.OpenFile(args[0], os.O_RDONLY, 0644)
			cobra.CheckErr(err)
			defer file.Close()
			br := bufio.NewReader(file)
			lineCounter := 0
			successCounter := 0
			failedCounter := 0
			ignoreCounter := 0
			for {
				line, err := br.ReadString('\n')
				if err != nil || io.EOF == err {
					break
				}
				lineCounter = lineCounter + 1
				if startLine > lineCounter {
					ignoreCounter = ignoreCounter + 1
					log.Infof("忽略第%v条命令:%v", lineCounter, line)
					continue
				}
				log.Infof("执行第%v条命令:%v", lineCounter, line)
				session, err := client.NewSession()
				if err != nil {
					failedCounter = failedCounter + 1
					log.Fatalf("执行第%v条命令失败,输出为:\n%v", lineCounter, err)
				}
				var stdout, stderr bytes.Buffer
				session.Stdout = &stdout
				session.Stderr = &stderr
				err = session.Run("/bin/bash -c " + line)
				if err != nil {
					failedCounter = failedCounter + 1
					if stderr.Len() > 0 {
						log.Warnf("执行第%v条命令失败,输出为:\n%s", lineCounter, stderr.String())
					} else {
						log.Warnf("执行第%v条命令失败", lineCounter)
					}
					if fastFail {
						break
					}
				} else {
					successCounter = successCounter + 1
					if stdout.Len() > 0 {
						log.Infof("执行第%v条命令成功,输出为:\n%s", lineCounter, stdout.String())
					} else {
						log.Infof("执行第%v条命令成功", lineCounter)
					}
				}
				session.Close()
			}
			log.Info("**********执行结束***********")
			log.Infof(
				"总计执行命令:%v条,成功:%v条,失败:%v条,忽略:%v条", lineCounter, successCounter, failedCounter,
				ignoreCounter)
		},
	}
	sshBatchCmd.Flags().Int("start-line", 0, "命令行开始位置")
	sshBatchCmd.Flags().Bool("fast-fail", true, "命令失败是否继续执行")

	sshBatchCmd.Flags().String("addr", "127.0.0.1:22", "服务器地址和端口")
	sshBatchCmd.MarkFlagRequired("addr")
	sshBatchCmd.Flags().String("user", "root", "用户名")
	sshBatchCmd.MarkFlagRequired("user")
	osCmd.AddCommand(sshBatchCmd)
}

// 保持cpu使用率
func keepCpu(targetPercent, deltaPercent float64, ctx context.Context) error {
	logicalCpus, err := cpu.Counts(true)
	cobra.CheckErr(err)
	totalMillis := logicalCpus * 1000
	targetMillis := float64(totalMillis) * targetPercent
	deltaMillis := float64(totalMillis) * deltaPercent
	go func() {
		var lastDeltaMillis float64
	forEnd:
		for {
			select {
			case <-ctx.Done():
				break forEnd
			default:
				startedTime := time.Now().UnixMilli()
				percents, err := cpu.Percent(0, false)
				cobra.CheckErr(err)
				for _, percent := range percents {
					log.Infof("curren cpu use: %v %%\n", percent/100.0)
				}
				currentPercent := percents[0] / 100.0
				currentMillis := float64(totalMillis) * currentPercent

				if (currentMillis-lastDeltaMillis)+deltaMillis < targetMillis {
					lastDeltaMillis = targetMillis - deltaMillis*rand.Float64() - currentMillis + lastDeltaMillis
					lastDeltaCountPerCpu := int64(lastDeltaMillis) / int64(logicalCpus)
					for i := 0; i < logicalCpus; i++ {
						go func() {
							select {
							case <-ctx.Done():
								return
							default:
								startedTime := time.Now().UnixMilli()
								for (time.Now().UnixMilli() - startedTime) < lastDeltaCountPerCpu {
								}
								sleepMills := 1000 - (time.Now().UnixMilli() - startedTime)
								if sleepMills <= 0 {
									time.Sleep(0)
								} else {
									time.Sleep(time.Duration(sleepMills) * time.Millisecond)
								}
							}
						}()
					}
				}

				sleepMills := 1000 - (time.Now().UnixMilli() - startedTime)
				if sleepMills <= 0 {
					time.Sleep(0 * time.Millisecond)
				} else {
					time.Sleep(time.Duration(sleepMills) * time.Millisecond)
				}
			}
		}
	}()
	return nil
}

// 保持mem使用率
func keepMem(targetPercent, deltaPercent float64, ctx context.Context) error {
	go func() {
		var sl []byte
		ticker := time.NewTicker(1 * time.Second)
	forEnd:
		for {
			select {
			case <-ctx.Done():
				break forEnd
			case <-ticker.C:
				memory, err := mem.VirtualMemory()
				cobra.CheckErr(err)
				log.Infof(
					"Total: %v,Used:%v,Available:%v, Free:%v, UsedPercent:%f %%\n",
					units.HumanSize(float64(memory.Total)), units.HumanSize(float64(memory.Used)),
					units.HumanSize(float64(memory.Available)), units.HumanSize(float64(memory.Free)),
					memory.UsedPercent)
				currentPercent := memory.UsedPercent / 100.0
				if currentPercent > (targetPercent + deltaPercent) { //高于上限
					sl = make([]byte, 0, 0)
					log.Infof("reduce to: %v\n", units.HumanSize(0))
				} else if currentPercent < (targetPercent - deltaPercent) { //低于下限
					memSize := (targetPercent - currentPercent - deltaPercent*rand.Float64()) * float64(memory.Total)
					sl = make([]byte, 0, int(memSize))
					log.Infof("adjust to: %v\n", units.HumanSize(memSize))
				}
			}
		}
		Unused(sl)
		return
	}()
	return nil
}
