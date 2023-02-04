package cmd

import (
	"context"
	"fmt"
	"github.com/docker/go-units"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/spf13/cobra"
	"math/rand"
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
			ctx := context.Background()
			keepCpu(0.7, 0.1, ctx)
			keepMem(0.8, 0.1, ctx)
			<-ctx.Done()
		},
	}
	resourceCmd.Flags().Float64("cpu", 0, "")
	resourceCmd.Flags().Float64("cpuP", 0, "")
	resourceCmd.Flags().Float64("mem", 0, "")
	resourceCmd.Flags().Float64("memP", 0, "")
	osCmd.AddCommand(resourceCmd)
}

//保持cpu使用率
func keepCpu(targetPercent, deltaPercent float64, ctx context.Context) error {

	physicalCounts, err := cpu.Counts(false)
	cobra.CheckErr(err)
	Unused(physicalCounts)
	logicalCounts, err := cpu.Counts(true)
	cobra.CheckErr(err)
	totalCounts := logicalCounts * 1000
	go func() {
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
					fmt.Printf("cpu: %v %%\n", percent/100.0)
				}
				currentPercent := percents[0] / 100.0

				if currentPercent < targetPercent-deltaPercent {
					averageDeltaCounts := int64((targetPercent-deltaPercent*rand.Float64()-currentPercent)*float64(totalCounts)) / int64(logicalCounts)

					for i := 0; i < logicalCounts; i++ {
						go func() {
							select {
							case <-ctx.Done():
								return
							default:
								startedTime := time.Now().UnixMilli()
								for (time.Now().UnixMilli() - startedTime) < averageDeltaCounts {
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

//保持mem使用率
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
				fmt.Printf("Total: %v,Used:%v,Available:%v, Free:%v, UsedPercent:%f %%\n",
					units.HumanSize(float64(memory.Total)), units.HumanSize(float64(memory.Used)),
					units.HumanSize(float64(memory.Available)), units.HumanSize(float64(memory.Free)),
					memory.UsedPercent)
				currentPercent := memory.UsedPercent / 100.0
				if currentPercent > (targetPercent + deltaPercent) { //高于上限
					sl = make([]byte, 0, 0)
					fmt.Printf("reduce to: %v\n", units.HumanSize(0))
				} else if currentPercent < (targetPercent - deltaPercent) { //低于下限
					memSize := (targetPercent - currentPercent - deltaPercent*rand.Float64()) * float64(memory.Total)
					sl = make([]byte, 0, int(memSize))
					fmt.Printf("adjust to: %v\n", units.HumanSize(memSize))
				} else {
					fmt.Printf("do thing \n")
				}
			}
		}
		Unused(sl)
		return
	}()
	return nil
}
