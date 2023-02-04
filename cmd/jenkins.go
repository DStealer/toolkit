package cmd

import (
	"bytes"
	"fmt"
	"github.com/docker/go-units"
	"github.com/prometheus/common/log"
	"github.com/spf13/cobra"
	"io/ioutil"
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
	client = http.Client{}
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
			proxy, err := cmd.Flags().GetString("proxy")
			cobra.CheckErr(err)

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
						log.Infof("fetch data from cache size:%s", units.HumanSize(float64(len(data))))
						return
					} else {
						expiringCache.Delete("/update-center.json")
					}
				}
				requestUrl := baseUrl + "/updates/update-center.json"
				log.Infof("fetch data from :%s", requestUrl)
				response, err := http.Get(requestUrl)
				if err != nil {
					log.Error("客户请求错误:", err)
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
				defer response.Body.Close()
				buf := new(bytes.Buffer)
				_, err = buf.ReadFrom(response.Body)
				if err != nil {
					log.Error("服务器转发错误:", err)
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
				if !(response.StatusCode >= 200 && response.StatusCode < 300) {
					log.Error("服务器响应错误:", response.Status)
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
				body := buf.String()
				body = strings.ReplaceAll(body, "https://www.google.com/", "https://www.baidu.com/")
				if proxy != "" {
					body = strings.ReplaceAll(body, "https://updates.jenkins.io/download", proxy)
				} else {
					body = strings.ReplaceAll(body, "https://updates.jenkins.io/download", baseUrl)
				}

				writer.WriteHeader(response.StatusCode)
				CopyHeader(response.Header, writer.Header())
				data := []byte(body)
				expiringCache.Set("/update-center.json", data, 24*time.Hour)
				_, err = writer.Write(data)
				if err != nil {
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
			})
			if proxy != "" {
				mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
					defer request.Body.Close()
					newRequest, err := http.NewRequest(request.Method, baseUrl+request.URL.String(), request.Body)
					log.Infof("fetch data from :%s", newRequest.URL)
					if err != nil {
						http.Error(writer, err.Error(), http.StatusInternalServerError)
						return
					}
					CopyHeader(request.Header, newRequest.Header, "Host")
					response, err := client.Do(newRequest)
					if err != nil {
						log.Error("请求错误:", request.Method, request.URL, err)
						http.Error(writer, err.Error(), http.StatusInternalServerError)
						return
					}
					writer.WriteHeader(response.StatusCode)
					defer response.Body.Close()
					CopyHeader(response.Header, writer.Header())
					bts, err := ioutil.ReadAll(response.Body)
					if err != nil {
						log.Error("请求错误:", request.Method, request.URL, err)
						http.Error(writer, err.Error(), http.StatusInternalServerError)
						return
					}
					_, err = writer.Write(bts)
					if err != nil {
						http.Error(writer, err.Error(), http.StatusInternalServerError)
					}
				})
			}
			log.Infof("注册jenkins插件中心:%s", "/update-center.json")

			addr := fmt.Sprintf("%s:%d", ip, port)
			log.Infof("服务器启动监听:%s", addr)
			err = http.ListenAndServe(addr, mux)
			log.Fatalln(err)
		},
	}

	ucCmd.Flags().IP("ip", net.ParseIP("127.0.0.1"), "绑定ip地址")
	ucCmd.Flags().Int("port", 8080, "绑定port")
	ucCmd.Flags().String("mirror", "tencent", " 选项: tencent huawei tsinghua ustc bit ")
	ucCmd.Flags().String("proxy", "", "自定义地址下载地址,如果不填写,则直接从原始地址下载,例如 http://127.0.0.1:8080/download")

	jenkinsCmd.AddCommand(ucCmd)
}
