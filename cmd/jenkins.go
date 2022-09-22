package cmd

import (
	"bytes"
	"fmt"
	"github.com/prometheus/common/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/cache"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	jenkinsCmd = &cobra.Command{
		Use:   "jenkins subcommand [args]",
		Short: "jenkins辅助命令",
	}
)

func init() {
	ucCmd := &cobra.Command{
		Use:   "uc [args]",
		Short: "启动 jenkins update center proxy",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			ip, err := cmd.Flags().GetIP("ip")
			cobra.CheckErr(err)
			port, err := cmd.Flags().GetInt("port")
			cobra.CheckErr(err)
			backend, err := cmd.Flags().GetString("mirror")
			cobra.CheckErr(err)

			mirrorDict := make(map[string]string)
			mirrorDict["tencent"] = "https://mirrors.cloud.tencent.com/jenkins"
			mirrorDict["huawei"] = "https://mirrors.huaweicloud.com/jenkins"
			mirrorDict["tsinghua"] = "https://mirrors.tuna.tsinghua.edu.cn/jenkins"
			mirrorDict["ustc"] = "https://mirrors.ustc.edu.cn/jenkins"
			mirrorDict["bit"] = "https://mirror.bit.edu.cn/jenkins"
			baseUrl, ok := mirrorDict[backend]
			if !ok {
				cobra.CheckErr("未识别的镜像地址")
			}

			mux := http.NewServeMux()
			expiringCache := cache.NewExpiring()
			mux.HandleFunc("/update-center.json", func(writer http.ResponseWriter, request *http.Request) {
				value, ok := expiringCache.Get("/update-center.json")
				if ok {
					data, ok := value.([]byte)
					if ok {
						writer.WriteHeader(http.StatusOK)
						writer.Header().Set("Content-Type", "application/json; charset=utf-8")
						_, _ = writer.Write(data)
						log.Infof("fetch data from cache size:%d",len(data))
						return
					} else {
						expiringCache.Delete("/update-center.json")
					}
				}
				originalUrl := fmt.Sprintf("%s/updates/update-center.json", baseUrl)
				log.Infof("fetch data from original:%s", originalUrl)
				resp, err := http.Get(originalUrl)
				if err != nil {
					log.Error("客户请求错误:", err)
					writer.WriteHeader(http.StatusBadRequest)
					writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
					_, _ = writer.Write([]byte("客户请求错误" + err.Error()))
					return
				}
				defer resp.Body.Close()
				buf := new(bytes.Buffer)
				_, err = buf.ReadFrom(resp.Body)
				if err != nil {
					log.Error("服务器转发错误:", err)
					writer.WriteHeader(http.StatusBadGateway)
					writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
					_, _ = writer.Write([]byte("服务器转发错误" + err.Error()))
					return
				}
				if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
					log.Error("服务器响应错误:", resp.Status)
					writer.WriteHeader(resp.StatusCode)
					_ = resp.Header.Write(writer)
					_, _ = writer.Write(buf.Bytes())
					return
				}
				body := buf.String()
				body = strings.ReplaceAll(body, "https://www.google.com/", "https://www.baidu.com/")
				body = strings.ReplaceAll(body, "https://updates.jenkins.io/download", baseUrl)

				writer.WriteHeader(http.StatusOK)
				writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				data := []byte(body)
				expiringCache.Set("/update-center.json", data, 24*time.Hour)
				_, err = writer.Write(data)
				if err != nil {
					log.Error("写入body错误:", err.Error())
					return
				}
			})
			log.Infof("注册jenkins插件中心:%s", "/update-center.json")

			addr := fmt.Sprintf("%s:%d", ip, port)
			log.Infof("服务器启动监听:%s", addr)
			err = http.ListenAndServe(addr, mux)
			log.Fatalln(err)
		},
	}

	ucCmd.Flags().IP("ip", net.ParseIP("127.0.0.1"), "绑定ip地址")
	ucCmd.Flags().Int("port", 8080, "绑定port")
	ucCmd.Flags().String("mirror", "tencent", "one of tencent huawei tsinghua ustc bit ")

	jenkinsCmd.AddCommand(ucCmd)
}
