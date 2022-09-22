package cmd

import (
	"bufio"
	"fmt"
	"github.com/bgentry/speakeasy"
	"github.com/dstealer/devops/pkg/hashtag"
	"github.com/go-redis/redis"
	"github.com/spf13/cobra"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

var (
	server   = "127.0.0.1:6379"
	db       = 0
	password = ""
	redisCmd = &cobra.Command{
		Use:   "redis subcommand [args]",
		Short: "redis运维管理工具",
	}
)

func init() {
	redisCmd.PersistentFlags().StringVar(&server, "server", server, "服务地址,一个地址为主从模式,多个地址为集群模式,哨兵模式暂时不支持")
	redisCmd.PersistentFlags().StringVar(&password, "password", password, "密码")
	redisCmd.PersistentFlags().IntVar(&db, "db", db, "数据库编号,仅主从模式,哨兵模式支持")
	redisCmd.AddCommand(&cobra.Command{
		Use:   "keys [key]",
		Short: "查看redis键的情况,支持模糊匹配",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := newRedisClient()
			defer client.Close()
			switch client.(type) {
			case *redis.ClusterClient:
				var hit int64 = 0
				var lck sync.Mutex
				err := client.(*redis.ClusterClient).ForEachMaster(func(client *redis.Client) error {
					iter := client.Scan(0, args[0], 100).Iterator()
					for iter.Next() {
						if iter.Err() != nil {
							return iter.Err()
						}
						lck.Lock()
						if atomic.AddInt64(&hit, 1) == 10 {
							fmt.Println("数量超出限制回复yes继续:")
							var confirm string
							_, err := fmt.Scanln(&confirm)
							if err != nil || !strings.EqualFold("yes", confirm) {
								fmt.Println("选择退出")
								os.Exit(0)
							}
						}
						lck.Unlock()
						fmt.Println(iter.Val())
					}
					return nil
				})
				cobra.CheckErr(err)
				fmt.Printf("命中key:%d\n", atomic.LoadInt64(&hit))
			default:
				iter := client.Scan(0, args[0], 100).Iterator()
				hit := 0
				for iter.Next() {
					hit++
					if hit == 10 {
						fmt.Print("redis集合数量超出限制回复yes继续获取其他退出操作:")
						var confirm string
						_, err := fmt.Scanln(&confirm)
						if err != nil || !strings.EqualFold("yes", confirm) {
							fmt.Println("选择退出")
							os.Exit(0)
						}
					}
					fmt.Println(iter.Val())
				}
				fmt.Printf("命中key:%d\n", hit)
			}
		},
	})

	redisCmd.AddCommand(&cobra.Command{
		Use:   "del [key]",
		Short: "删除redis中键,支持模糊匹配",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("高危操作! 确认删除:[%s]\n请输入yes:继续操作 ", args[0])
			var confirm string
			_, err := fmt.Scanln(&confirm)
			if err != nil || !strings.EqualFold("yes", confirm) {
				fmt.Println("选择退出操作")
				os.Exit(0)
			}
			client := newRedisClient()
			defer client.Close()
			//模糊匹配
			if strings.ContainsAny(args[0], "*[]?") {
				switch client.(type) {
				case *redis.ClusterClient:
					var hit int64 = 0
					err := client.(*redis.ClusterClient).ForEachMaster(func(client *redis.Client) error {
						iter := client.Scan(0, args[0], 100).Iterator()
						keys := make([]string, 0)
						for iter.Next() {
							key := iter.Val()
							if len(keys) > 200 || (len(keys) > 0 && hashtag.Slot(keys[0]) != hashtag.Slot(key)) {
								affected, err := client.Del(keys...).Result()
								cobra.CheckErr(err)
								atomic.AddInt64(&hit, affected)
								fmt.Printf("清理缓存数量:%d\n", hit)
								keys = make([]string, 0)
							}
							keys = append(keys, key)
						}
						if len(keys) > 0 {
							fmt.Printf("清理缓存数量:%d\n", hit)
							affected, err := client.Del(keys...).Result()
							cobra.CheckErr(err)
							atomic.AddInt64(&hit, affected)
						}
						return nil
					})
					cobra.CheckErr(err)
					fmt.Printf("命中key:%d\n", atomic.LoadInt64(&hit))
				default:
					iter := client.Scan(0, args[0], 100).Iterator()
					keys := make([]string, 0)
					var hit int64 = 0
					for iter.Next() {
						key := iter.Val()
						if len(keys) > 200 {
							affected, err := client.Del(keys...).Result()
							cobra.CheckErr(err)
							hit += affected
							fmt.Printf("清理缓存数量:%d\n", hit)
							keys = make([]string, 0)
						}
						keys = append(keys, key)
					}
					if len(keys) > 0 {
						fmt.Printf("清理缓存数量:%d\n", hit)
						affected, err := client.Del(keys...).Result()
						cobra.CheckErr(err)
						hit += affected
					}
					fmt.Printf("总共清理缓存数量:%d\n", hit)
				}
			} else {
				affected, err := client.Del(args[0]).Result()
				cobra.CheckErr(err)
				fmt.Printf("总共清理缓存数量:%d\n", affected)
			}
		},
	})
	redisCmd.AddCommand(&cobra.Command{
		Use:   "type [key]",
		Short: "查看redis中键信息",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := newRedisClient()
			defer client.Close()
			result, err := client.Type(args[0]).Result()
			cobra.CheckErr(err)
			duration, err := client.TTL(args[0]).Result()
			cobra.CheckErr(err)
			encoding, err := client.ObjectEncoding(args[0]).Result()
			cobra.CheckErr(err)
			fmt.Printf("键: [%s]\n类型: [%s]\n存活时间: [%v]\n编码: [%v]\n", args[0], result, duration, encoding)
			switch result {
			case "string":
				{
					length, err := client.StrLen(args[0]).Result()
					cobra.CheckErr(err)
					fmt.Printf("长度:[%v]\n", length)
				}
			case "hash":
				{
					length, err := client.HLen(args[0]).Result()
					cobra.CheckErr(err)
					fmt.Printf("元素数量:[%v]\n", length)
				}
			case "list":
				{
					length, err := client.LLen(args[0]).Result()
					cobra.CheckErr(err)
					fmt.Printf("元素数量:[%v]\n", length)
				}
			case "set":
				{
					length, err := client.SCard(args[0]).Result()
					cobra.CheckErr(err)
					fmt.Printf("元素数量:[%v]\n", length)
				}
			case "zset":
				{
					length, err := client.ZCard(args[0]).Result()
					cobra.CheckErr(err)
					fmt.Printf("元素数量:[%v]\n", length)
				}
			default:
				fmt.Printf("unknown type:[%v]\n", result)
			}
		},
	})

	redisCmd.AddCommand(&cobra.Command{
		Use:   "clean [principal file]",
		Short: "清理app用户会话",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := newRedisClient()
			defer client.Close()
			file, err := os.Open(args[0])
			cobra.CheckErr(err)
			defer file.Close()
			reader := bufio.NewReader(file)
			var hit int32
			for {
				line, _, err := reader.ReadLine()
				if err == io.EOF {
					break
				} else {
					cobra.CheckErr(err)
				}
				principal := strings.TrimFunc(string(line), func(r rune) bool {
					return r == '"' || unicode.IsSpace(r)
				})
				indexKeys := fmt.Sprintf("spring:session:index:org.springframework.session.FindByIndexNameSessionRepository.PRINCIPAL_NAME_INDEX_NAME:app:%s", principal)
				sessions, err := client.SMembers(indexKeys).Result()
				cobra.CheckErr(err)
				for _, session := range sessions {
					fmt.Printf("清理用户:%s 会话:%s\n", principal, session)
					_, err = client.Del(fmt.Sprintf("spring:session:sessions:expires:%s", strings.Trim(session, "\""))).Result()
					cobra.CheckErr(err)
					_, err = client.Del(fmt.Sprintf("spring:session:sessions:%s", strings.Trim(session, "\""))).Result()
					cobra.CheckErr(err)
					hit += 1
				}
				if len(sessions) > 0 {
					_, err = client.Del(indexKeys).Result()
					cobra.CheckErr(err)
				}
			}
			fmt.Println("处理完成,处理记录", hit, "条")
		},
	})
}

func newRedisClient() redis.UniversalClient {
	if len(password) == 0 {
		psd, err := speakeasy.Ask("请输入密码:")
		cobra.CheckErr(err)
		password = psd
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        strings.Split(server, ","),
		DB:           db,
		Password:     password,
		MaxRetries:   5,
		PoolSize:     runtime.NumCPU(),
		MaxRedirects: 10,
	})
	_, err := client.Ping().Result()
	cobra.CheckErr(err)
	return client
}
