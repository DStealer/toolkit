package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "tk",
	Short:   "运维工具箱",
	Version: "v0.0.2",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(osCmd)
	rootCmd.AddCommand(redisCmd)
	rootCmd.AddCommand(sm4Cmd)
	rootCmd.AddCommand(jarCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(jenkinsCmd)
}
