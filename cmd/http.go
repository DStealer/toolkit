package cmd

import (
	auth "github.com/abbot/go-http-auth"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	"net"
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
		Short: "http basic auth",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			provider := auth.HtpasswdFileProvider(args[0])
			upstream, err := url.ParseRequestURI(args[1])
			cobra.CheckErr(err)
			authenticator := auth.NewBasicAuthenticator(upstream.Hostname(), provider)
			http.HandleFunc("/", authenticator.Wrap(reverseProxy(*upstream)))
			err = http.ListenAndServe("127.0.0.1:8080", nil)
			cobra.CheckErr(err)
		},
	}
	authProxyCmd.Flags().Int32("local-port", 8080, "本地绑定端口")
	authProxyCmd.Flags().IP("local-ip", net.ParseIP(GetLocalIP()), "本地绑定端口")
	authProxyCmd.Flags().String("upstream", "http://127.0.0.1:8080", "上游代理地址")

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
