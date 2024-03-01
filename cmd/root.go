package cmd

import (
	"github.com/spf13/cobra"
)

var (
	Version  = "No Version Provided"
	Compile  = ""
	Branch   = ""
	GitDirty = ""
)

var rootCmd = &cobra.Command{
	Use:     "tk",
	Short:   "运维工具箱",
	Version: Version,
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
	rootCmd.AddCommand(k8sCmd)
	rootCmd.AddCommand(mysqlCmd)
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(nodeJsCmd)
}
