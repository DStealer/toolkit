package cmd

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"github.com/siddontang/go-log/log"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/homedir"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

var (
	k8sCmd = &cobra.Command{
		Use:   "k8s subcommand [args]",
		Short: "k8s辅助命令",
	}
)

func init() {

	k8sCmd.PersistentFlags().String(
		"kubeconfig", filepath.Join(homedir.HomeDir(), ".kube", "config"), "kubectl的配置文件路径")
	k8sCmd.PersistentFlags().String("context", "", "当前使用的上下文环境")
	k8sCmd.PersistentFlags().String("namespace", "default", "当前使用的命名空间")

	sshCmd := &cobra.Command{
		Use:   "ssh [args]",
		Short: "ssh 网络工具",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			stopChannel := make(chan struct{}, 1)
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt)
			defer signal.Stop(signals)
			go func() {
				<-signals
				if stopChannel != nil {
					close(stopChannel)
				}
			}()
			kubeconfig, err := cmd.Flags().GetString("kubeconfig")
			cobra.CheckErr(err)

			currentContext, err := cmd.Flags().GetString("context")
			cobra.CheckErr(err)

			namespace, err := cmd.Flags().GetString("namespace")
			cobra.CheckErr(err)

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
				&clientcmd.ConfigOverrides{CurrentContext: currentContext, ClusterInfo: clientcmdapi.Cluster{InsecureSkipTLSVerify: true}})

			config, err := clientConfig.ClientConfig()
			cobra.CheckErr(err)

			clientSet := kubernetes.NewForConfigOrDie(config)

			pod, err := clientSet.CoreV1().Pods(namespace).Get(context.TODO(), "netshoot", metav1.GetOptions{})

			if errors.IsNotFound(err) {
				image, err := cmd.Flags().GetString("image")
				cobra.CheckErr(err)
				pod = &v1.Pod{
					TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "netshoot",
						Namespace: "default",
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{
							Name:            "app",
							Image:           image,
							ImagePullPolicy: v1.PullIfNotPresent,
							Ports: []v1.ContainerPort{
								{Name: "ssh", ContainerPort: 22, Protocol: v1.ProtocolTCP},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("ssh"),
									},
								},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("ssh"),
									},
								},
							},
							StartupProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("ssh"),
									},
								},
							},
							Resources: v1.ResourceRequirements{
								Requests: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("0.1"), v1.ResourceMemory: resource.MustParse("128Mi")},
								Limits:   map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("512Mi")},
							},
						}},
					},
				}
				_, err = clientSet.CoreV1().Pods(pod.Namespace).Create(
					context.Background(), pod, metav1.CreateOptions{})
				cobra.CheckErr(err)

				err = wait.PollImmediateUntil(
					1*time.Second, func() (bool, error) {
						newPod, err := clientSet.CoreV1().Pods(pod.Namespace).Get(
							context.TODO(), pod.Name, metav1.GetOptions{})
						if err != nil {
							fmt.Fprintf(os.Stderr, "Error getting Pod :%q [%v]\n", newPod.Name, err)
							return false, nil
						}
						if newPod == nil {
							fmt.Fprintf(os.Stderr, "Pod :%q not found\n", newPod.Name)
							return false, nil
						}
						if newPod.Status.Phase != v1.PodRunning {
							return false, nil
						}
						return true, nil
					}, stopChannel)

				cobra.CheckErr(err)
				pod, err = clientSet.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
				cobra.CheckErr(err)
			}

			if pod.Status.Phase != v1.PodRunning {
				cobra.CheckErr(fmt.Sprintf("unable to forward port because pod is not running. Current status=%v", pod))
			}

			req := clientSet.CoreV1().RESTClient().Post().
				Resource("pods").
				Namespace(pod.Namespace).
				Name(pod.Name).
				SubResource("portforward")

			transport, upgrader, err := spdy.RoundTripperFor(config)
			cobra.CheckErr(err)
			dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

			readyChannel := make(chan struct{})
			localPort := cmd.Flag("local-port").Value.String()
			fw, err := portforward.NewOnAddresses(
				dialer, []string{"127.0.0.1"}, []string{fmt.Sprintf("%v:22", localPort)}, stopChannel, readyChannel,
				os.Stdout, os.Stderr)
			cobra.CheckErr(err)
			log.Infof("ssh port listen on 127.0.0.1:%v \n", localPort)
			err = fw.ForwardPorts()
			cobra.CheckErr(err)
		},
	}
	sshCmd.Flags().String("image", "registry.develop.com:5000/library/netshoot-sshd:latest", "使用的镜像")
	sshCmd.Flags().Int("local-port", 22622, "使用的本地端口")
	k8sCmd.AddCommand(sshCmd)

	jarLibCmd := &cobra.Command{
		Use:   "jarlib csvpath [args]",
		Short: "jar lib 分析工具",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			stopChannel := make(chan struct{}, 1)
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt)
			defer signal.Stop(signals)
			go func() {
				<-signals
				if stopChannel != nil {
					close(stopChannel)
				}
			}()
			kubeconfig, err := cmd.Flags().GetString("kubeconfig")
			cobra.CheckErr(err)

			currentContext, err := cmd.Flags().GetString("context")
			cobra.CheckErr(err)

			namespace, err := cmd.Flags().GetString("namespace")
			cobra.CheckErr(err)

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
				&clientcmd.ConfigOverrides{CurrentContext: currentContext, ClusterInfo: clientcmdapi.Cluster{InsecureSkipTLSVerify: true}})

			config, err := clientConfig.ClientConfig()
			cobra.CheckErr(err)

			clientSet := kubernetes.NewForConfigOrDie(config)

			JarImageLibs := make([]JarImageLib, 0)
			podList, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
			cobra.CheckErr(err)
			for _, pod := range podList.Items {
				if pod.Status.Phase != v1.PodRunning {
					log.Warnf("pod %s is not running\n", pod.Name)
					continue
				}
				log.Infof("pod %s is running\n", pod.Name)
				// 构造执行命令请求
				req := clientSet.CoreV1().RESTClient().Post().
					Resource("pods").
					Name(pod.Name).
					Namespace(pod.Namespace).
					SubResource("exec").
					VersionedParams(
						&v1.PodExecOptions{
							Command: []string{"sh", "-c", "find /app -name *.jar -print0 | xargs -0 md5sum"},
							Stdin:   true,
							Stdout:  true,
							Stderr:  true,
							TTY:     false,
						}, scheme.ParameterCodec)
				// 执行命令
				executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
				cobra.CheckErr(err)
				// 使用bytes.Buffer变量接收标准输出和标准错误
				var stdout, stderr bytes.Buffer
				err = executor.Stream(
					remotecommand.StreamOptions{
						Stdin:  strings.NewReader(""),
						Stdout: &stdout,
						Stderr: &stderr,
					})
				if err != nil {
					log.Warnf("标准错误:\n%s\n", stderr.String())
					continue
				}
				podImages := make([]string, 0)
				for _, c := range pod.Spec.Containers {
					podImages = append(podImages, c.Image)
				}
				jarLibs := parseJarLibFromStdout(stdout.String())

				jarImageLib := JarImageLib{
					PodName:   pod.Name,
					Namespace: namespace,
					Images:    podImages,
					JarLibs:   jarLibs,
				}
				log.Infof("解析namespace:%s name:%s lib:%d\n", pod.Namespace, pod.Name, len(jarLibs))
				JarImageLibs = append(JarImageLibs, jarImageLib)
			}
			file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
			cobra.CheckErr(err)
			defer file.Close()
			csvWriter := csv.NewWriter(file)
			csvWriter.Write([]string{"命名空间", "pod名称", "镜像信息", "依赖名称", "依赖路径", "依赖MD5"})
			for _, jarImageLib := range JarImageLibs {
				for _, jarLib := range jarImageLib.JarLibs {
					csvWriter.Write(
						[]string{jarImageLib.Namespace, jarImageLib.PodName, strings.Join(
							jarImageLib.Images, ","), jarLib.JarName, jarLib.JarPath, jarLib.Md5})
				}
			}
			csvWriter.Flush()
			if absPath, err := filepath.Abs(args[0]); err == nil {
				log.Infof("写入csv文件:%s完成,总计:%d\n", absPath, len(JarImageLibs))
			} else {
				log.Infof("写入csv文件:%s完成,总计:%d\n", file.Name(), len(JarImageLibs))
			}

		},
	}
	k8sCmd.AddCommand(jarLibCmd)

	imageListCmd := &cobra.Command{
		Use:   "imagelist [args]",
		Short: "运行镜像分析工具",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			stopChannel := make(chan struct{}, 1)
			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt)
			defer signal.Stop(signals)
			go func() {
				<-signals
				if stopChannel != nil {
					close(stopChannel)
				}
			}()
			kubeconfig, err := cmd.Flags().GetString("kubeconfig")
			cobra.CheckErr(err)

			currentContext, err := cmd.Flags().GetString("context")
			cobra.CheckErr(err)

			namespace, err := cmd.Flags().GetString("namespace")
			cobra.CheckErr(err)

			clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
				&clientcmd.ConfigOverrides{CurrentContext: currentContext, ClusterInfo: clientcmdapi.Cluster{InsecureSkipTLSVerify: true}})

			config, err := clientConfig.ClientConfig()
			cobra.CheckErr(err)

			clientSet := kubernetes.NewForConfigOrDie(config)

			podImages := make([][]string, 0)
			podList, err := clientSet.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
			cobra.CheckErr(err)
			for _, pod := range podList.Items {
				if pod.Status.Phase != v1.PodRunning {
					log.Warnf("pod %s is not running\n", pod.Name)
					continue
				}
				log.Infof("pod %s is running\n", pod.Name)
				// 构造执行命令请求
				podImage := make([]string, 3)

				podImage = append(podImage, pod.Namespace)
				podImage = append(podImage, pod.Name)
				images := make([]string, 0)
				for _, c := range pod.Spec.Containers {
					images = append(images, c.Image)
				}
				podImage = append(podImage, strings.Join(images, ","))
			}

			if len(args) == 0 {
				csvWriter := csv.NewWriter(os.Stdout)
				csvWriter.Write([]string{"命名空间", "pod名称", "镜像信息"})
				for _, podImage := range podImages {
					csvWriter.Write(podImage[:])
				}
				csvWriter.Flush()
				log.Infof("写入完成,总计:%d\n", len(podImages))
			} else {
				file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
				cobra.CheckErr(err)
				defer file.Close()
				csvWriter := csv.NewWriter(file)
				csvWriter.Write([]string{"命名空间", "pod名称", "镜像信息"})
				for _, podImage := range podImages {
					csvWriter.Write(podImage[:])
				}
				csvWriter.Flush()
				if absPath, err := filepath.Abs(args[0]); err == nil {
					log.Infof("写入csv文件:%s完成,总计:%d\n", absPath, len(podImages))
				} else {
					log.Infof("写入csv文件:%s完成,总计:%d\n", file.Name(), len(podImages))
				}

			}

		},
	}
	k8sCmd.AddCommand(imageListCmd)

}

// 解析Md5标准输出
func parseJarLibFromStdout(stdout string) []JarLib {
	jarLibs := make([]JarLib, 0)

	for _, line := range strings.Split(stdout, "\n") {
		if len(line) == 0 {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			log.Warnf("invalid line: %s\n", line)
			continue
		}

		jarLib := JarLib{
			JarName: filepath.Base(strings.TrimSpace(parts[1])),
			JarPath: strings.TrimSpace(parts[1]),
			Md5:     strings.TrimSpace(parts[0]),
		}
		jarLibs = append(jarLibs, jarLib)
	}
	return jarLibs
}

type JarImageLib struct {
	PodName   string
	Namespace string
	Images    []string
	JarLibs   []JarLib
}
type JarLib struct {
	JarName string
	JarPath string
	Md5     string
}
