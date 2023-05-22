package cmd

import (
	"fmt"
	auth "github.com/abbot/go-http-auth"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var (
	httpCmd = &cobra.Command{
		Use:   "http subcommand [args]",
		Short: "http增强命令",
	}
)

func init() {
	authProxyCmd := &cobra.Command{
		Use:   "auth htpasswdfile",
		Short: "http basic auth, 仅支持htpasswd命令bcrypt encryption方式生成的文件",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			provider := auth.HtpasswdFileProvider(args[0])
			upstreamStr, err := cmd.Flags().GetString("upstreamAddr")
			cobra.CheckErr(err)
			upstream, err := url.ParseRequestURI(upstreamStr)
			cobra.CheckErr(err)
			authenticator := auth.NewBasicAuthenticator(upstream.Hostname(), provider)
			http.HandleFunc("/", authenticator.Wrap(reverseProxy(*upstream)))
			localAddr, err := cmd.Flags().GetString("localAddr")
			cobra.CheckErr(err)
			log.Infof("绑定本地:%s", localAddr)
			err = http.ListenAndServe(localAddr, nil)
			cobra.CheckErr(err)
		},
	}
	authProxyCmd.Flags().String("localAddr", fmt.Sprintf("%s:%d", GetLocalIP(), 8080), "本地绑定ip和端口")
	authProxyCmd.Flags().String("upstreamAddr", "http://127.0.0.1:8080", "上游代理地址")

	httpCmd.AddCommand(authProxyCmd)
}

// 反向代理实现
func reverseProxy(upstream url.URL) func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	proxy := httputil.ReverseProxy{
		Director: func(request *http.Request) {
			request.URL.Scheme = upstream.Scheme
			request.URL.Host = upstream.Host
			request.Host = upstream.Host
			log.Infof("access :%v", request.URL)
		},
	}
	return func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		proxy.ServeHTTP(w, &r.Request)
	}
}
