package cmd

import (
	"encoding/json"
	"github.com/prometheus/common/log"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var (
	nodeJsCmd = &cobra.Command{
		Use:   "nodejs subcommand [args]",
		Short: "nodejs运维管理工具",
	}
)

func init() {
	packageLockFixUpCmd := &cobra.Command{
		Use:   "packageLockFixUp [path expression]",
		Short: "修订package-lock.json文件",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath, err := filepath.Abs(args[0])
			cobra.CheckErr(err)
			file, err := os.ReadFile(filePath)
			cobra.CheckErr(err)

			registry, err := cmd.Flags().GetString("registry")
			cobra.CheckErr(err)

			packageLock := make(map[string]interface{}, 64)
			err = json.Unmarshal(file, &packageLock)
			cobra.CheckErr(err)
			lockfileVersion, ok := packageLock["lockfileVersion"]
			fixupped := false
			if !ok || cast.ToInt(lockfileVersion) == 1 { //npm v5  v6.
				if dependencies, ok := packageLock["dependencies"]; ok {
					for name, dependency := range dependencies.(map[string]interface{}) {
						fixupped = FixupResolvedRegistryV1(name, dependency.(map[string]interface{}), registry) || fixupped
					}
				}
			} else if cast.ToInt(lockfileVersion) == 2 { //npm v7 backwards compatible to v1 lockfiles.
				log.Warn("暂时不支持v2版本文件")
				fixupped = false
			} else if cast.ToInt(lockfileVersion) == 3 { //npm v7 without backwards compatibility
				if packages, ok := packageLock["packages"]; ok {
					for name, pkg := range packages.(map[string]interface{}) {
						fixupped = FixupResolvedRegistryV3(name, pkg.(map[string]interface{}), registry) || fixupped
					}
				}
			} else {
				log.Warn("不支持的文件")
				fixupped = false
			}
			if !fixupped {
				log.Info("文件无需修订")
				return
			}
			marshal, err := json.MarshalIndent(packageLock, "", "  ")
			cobra.CheckErr(err)
			log.Infof("备份文件:%s", filePath+".original")
			err = os.Rename(filePath, filePath+".original")
			cobra.CheckErr(err)
			err = os.WriteFile(filePath, marshal, 0x644)
			cobra.CheckErr(err)
			log.Info("文件修订完成")
		},
	}
	packageLockFixUpCmd.Flags().String("registry", "https://registry.npmmirror.com/", "npm镜像地址")

	nodeJsCmd.AddCommand(packageLockFixUpCmd)
}

func FixupResolvedRegistryV1(name string, dependency map[string]interface{}, registryString string) (fixed bool) {
	if name == "" || len(dependency) == 0 || registryString == "" {
		return
	}
	if resolved, ok := dependency["resolved"]; ok {
		if resolvedString, ok := resolved.(string); ok {
			resolvedString = strings.Replace(resolvedString, "/download/", "-", 1)
			if !strings.HasPrefix(resolvedString, registryString) {
				if strings.HasPrefix(resolvedString, "http") {
					index := strings.Index(resolvedString, name)
					if index > -1 {
						dependency["resolved"] = registryString + resolvedString[index:]
						fixed = true
						log.Infof("fixup %s to %s", resolvedString, registryString+resolvedString[index:])
					}
				} else {
					log.Warnf("can't fixup %s", resolvedString)
				}
			}
		}
	}
	if dependencies, ok := dependency["dependencies"]; ok {
		for name, dependency := range dependencies.(map[string]interface{}) {
			fixed = FixupResolvedRegistryV1(name, dependency.(map[string]interface{}), registryString) || fixed
		}
	}
	return fixed
}
func FixupResolvedRegistryV3(name string, dependency map[string]interface{}, registryString string) (fixed bool) {
	if name == "" || len(dependency) == 0 || registryString == "" {
		return
	}
	if resolved, ok := dependency["resolved"]; ok {
		if resolvedString, ok := resolved.(string); ok {
			if !strings.HasPrefix(resolvedString, registryString) {
				if strings.HasPrefix(resolvedString, "http") {
					nameIndex := strings.LastIndex(name, "node_modules/") + len("node_modules/")
					name = name[nameIndex:]
					index := strings.Index(resolvedString, name)
					if index > -1 {
						dependency["resolved"] = registryString + resolvedString[index:]
						fixed = true
						log.Infof("fixup %s to %s", resolvedString, registryString+resolvedString[index:])
					}
				} else {
					log.Warnf("can't fixup %s", resolvedString)
				}
			}
		}
	}
	return fixed
}
