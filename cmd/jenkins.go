package cmd

import (
	"bytes"
	"fmt"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var (
	jenkinsCmd = &cobra.Command{
		Use:   "jenkins subcommand [args]",
		Short: "jenkins辅助命令",
	}
	jenkinsClient = http.Client{}
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
			mux.HandleFunc("/update-center.json", func(writer http.ResponseWriter, request *http.Request) {
				version := request.URL.Query().Get("version")
				var response *http.Response
				if version != "" {
					requestUrl := baseUrl + "/updates" + fmt.Sprintf("/dynamic-stable-%s", version) + request.RequestURI
					log.Infof("fetch data from :%s", requestUrl)
					response, err = http.Get(requestUrl)
					if err != nil {
						log.Error("客户请求错误:", err)
						http.Error(writer, err.Error(), http.StatusInternalServerError)
						return
					}
				} else {
					requestUrl := baseUrl + "/updates" + request.RequestURI
					log.Infof("fetch data from :%s", requestUrl)
					response, err = http.Get(requestUrl)
					if err != nil {
						log.Error("客户请求错误:", err)
						http.Error(writer, err.Error(), http.StatusInternalServerError)
						return
					}
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
				_, err = writer.Write(data)
				if err != nil {
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
				}
			})
			if proxy != "" {
				upstream, err := url.Parse(baseUrl)
				cobra.CheckErr(err)
				reverseProxy := httputil.ReverseProxy{
					Director: func(request *http.Request) {
						request.URL.Scheme = upstream.Scheme
						request.URL.Host = upstream.Host
						request.Host = upstream.Host
						path, err := url.JoinPath(upstream.Path, request.URL.Path)
						if err == nil {
							request.URL.Path = path
						}
						log.Debugf("access :%s", request.URL)
					},
				}

				mux.HandleFunc("/", reverseProxy.ServeHTTP)
			}

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
