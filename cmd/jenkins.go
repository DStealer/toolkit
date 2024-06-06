package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/bmatcuk/doublestar"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	jenkinsUcCmd := &cobra.Command{
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
			mux.HandleFunc(
				"/update-center.json", func(writer http.ResponseWriter, request *http.Request) {
					version := request.URL.Query().Get("version")
					var response *http.Response
					if version != "" {
						var requestUrl string
						if matched, err := regexp.MatchString("\\d+\\.\\d+\\.\\d+", version); err == nil && matched {
							requestUrl = baseUrl + fmt.Sprintf("/updates/dynamic-stable-%s/update-center.json", version)
						} else {
							versionNo := strings.Split(version, ".")
							requestUrl = baseUrl + fmt.Sprintf(
								"/updates/dynamic-%s.%s/update-center.json", versionNo[0], versionNo[1])
						}
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

	jenkinsUcCmd.Flags().IP("ip", net.ParseIP("127.0.0.1"), "绑定ip地址")
	jenkinsUcCmd.Flags().Int("port", 8080, "绑定port")
	jenkinsUcCmd.Flags().String("mirror", "tencent", " 选项: tencent huawei tsinghua ustc bit ")
	jenkinsUcCmd.Flags().String(
		"proxy", "", "自定义地址下载地址,如果不填写,则直接从原始地址下载,例如 http://127.0.0.1:8080/download")

	jenkinsCmd.AddCommand(jenkinsUcCmd)

	jenkinsGetPkgCmd := &cobra.Command{
		Use:   "getPkg pkg.txt buildDir destDir [args]",
		Short: "从pkg.txt中读取文件并从jenkins构建目录下获取jar包",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			jenkinsProjectGlob, err := cmd.Flags().GetString("jenkins-project-glob")
			cobra.CheckErr(err)

			permalinksGlobPattern := filepath.Join(args[1], jenkinsProjectGlob, "permalinks")
			matches, err := doublestar.Glob(permalinksGlobPattern)
			cobra.CheckErr(err)
			pkgInfoMap := make(map[string]GetPkgInfo)
			for _, match := range matches {
				permalinks, err := ParsePermalinks(match)
				if err != nil {
					log.Warnf("解析:%s失败", match)
					continue
				}
				log.Infof("开始解析项目:%s", match)
				lastSuccessfulBuild, ok := permalinks["lastSuccessfulBuild"]
				if !ok || lastSuccessfulBuild == "-1" || lastSuccessfulBuild == "0" {
					log.Warnf("项目:%s没有构建成功", match)
					continue
				}
				base := filepath.Dir(match)
				archiveGlob := filepath.Join(base, lastSuccessfulBuild, "archive", "**", "*.jar")
				archiveMatches, err := doublestar.Glob(archiveGlob)
				if err != nil {
					log.Warnf("解析构件地址:%s失败", archiveGlob)
					continue
				}
				for _, archiveMatch := range archiveMatches {
					name := filepath.Base(archiveMatch)
					md5Sum, err := Md5Sum(archiveMatch)
					cobra.CheckErr(err)
					pkgInfo := GetPkgInfo{Name: name, Project: "", Path: archiveMatch, Build: lastSuccessfulBuild, Md5: md5Sum}
					if oldPkgInfo, ok := pkgInfoMap[name]; !ok {
						pkgInfoMap[name] = pkgInfo
					} else {
						log.Warnf("项目%s和已有项目%s冲突程序无法自动处理", pkgInfo, oldPkgInfo)
					}
				}
			}
			fileHandler, err := os.OpenFile(args[0], os.O_RDONLY, 0666)
			cobra.CheckErr(err)
			defer fileHandler.Close()
			reader := bufio.NewReader(fileHandler)
			totalNum := 0
			succPkgcInfos := make([]GetPkgInfo, 0, 16)
			for {
				line, _, err := reader.ReadLine()
				if err == io.EOF {
					break
				}
				jarFileName := strings.TrimSpace(string(line))
				if jarFileName == "" {
					continue
				}
				if strings.HasPrefix(jarFileName, "#") {
					log.Infof("忽略注释行:%s", jarFileName)
					continue
				}

				totalNum = totalNum + 1
				log.Infof("处理第个文件:%s", totalNum, jarFileName)
				if pkgInfo, ok := pkgInfoMap[jarFileName]; ok {
					log.Infof("包信息:%v", pkgInfo)
					srcFile, err := os.OpenFile(pkgInfo.Path, os.O_RDONLY, 0644)
					cobra.CheckErr(err)
					defer srcFile.Close()
					dstFilePath := filepath.Join(args[2], jarFileName)
					dstFile, err := os.Create(dstFilePath)
					defer dstFile.Close()
					_, err = io.Copy(dstFile, srcFile)
					cobra.CheckErr(err)
					succPkgcInfos = append(succPkgcInfos, pkgInfo)
				} else {
					log.Warnf("没有找到文件:%s", jarFileName)
				}
			}
			log.Infof("统计信息:")
			for idx, pkg := range succPkgcInfos {
				fmt.Println(idx, " ", pkg.Name, " ", pkg.Project, " ", pkg.Build, " ", pkg.Md5, " ", pkg.Path)
			}
			log.Infof("处理完成,总计:%d,成功:%d", totalNum, len(succPkgcInfos))

		},
	}

	jenkinsGetPkgCmd.Flags().String(
		"jenkins-project-glob", "", "使用jenkins builds project模式解析,此时第一个参数应该为jenkins的builds目录")

	jenkinsCmd.AddCommand(jenkinsGetPkgCmd)

}

// 包信息
type GetPkgInfo struct {
	Name    string
	Project string
	Path    string
	Build   string
	Md5     string
}
