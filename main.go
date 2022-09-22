package main

import (
	"fmt"
	"os"
)
import "github.com/dstealer/devops/cmd"

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Println("执行失败-->", err.Error())
		os.Exit(1)
	}
}
