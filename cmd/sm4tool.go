package cmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/ZZMarquis/gm/sm4"
	"github.com/bgentry/speakeasy"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	sm4Cmd = &cobra.Command{
		Use:   "sm4 subcommand [args]",
		Short: "加解密管理工具",
	}
)

func init() {
	sm4Cmd.AddCommand(&cobra.Command{
		Use:   "genkey [key size]",
		Short: "生成密钥",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			rand.Seed(time.Now().UnixNano())
			size, err := strconv.Atoi(args[0])
			cobra.CheckErr(err)
			var buffer bytes.Buffer
			candidate := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
			for i := 0; i < size; i++ {
				buffer.WriteRune(candidate[rand.Intn(len(candidate))])
			}
			fmt.Printf("密码是: %s\n", buffer.String())
		},
	})

	sm4Cmd.AddCommand(&cobra.Command{
		Use:   "enc [plain text]",
		Short: "加密明文",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			plainByteArray := []byte(args[0])
			paddingPlainByteArray := append(plainByteArray, bytes.Repeat([]byte{byte(0x00)},
				16-len(plainByteArray)%16)...)
			encryptText, err := sm4.ECBEncrypt(getPassword(), paddingPlainByteArray)
			cobra.CheckErr(err)
			fmt.Printf("密文是: %s\n", hex.EncodeToString(encryptText))
		},
	})
	sm4Cmd.AddCommand(&cobra.Command{
		Use:   "dec [encrypt text]",
		Short: "解密密文",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			encryptByteArray, err := hex.DecodeString(args[0])
			cobra.CheckErr(err)
			plainByteArray, err := sm4.ECBDecrypt(getPassword(), encryptByteArray)
			cobra.CheckErr(err)
			fmt.Printf("明文是: %s\n", strings.TrimRightFunc(string(plainByteArray),
				func(r rune) bool { return r == 0x00 }))
		},
	})
}

func getPassword() []byte {
	password, err := speakeasy.Ask("请输入密钥:")
	cobra.CheckErr(err)
	if password == "" {
		fmt.Println("选择退出操作")
		os.Exit(0)
	}
	return []byte(password)
}
