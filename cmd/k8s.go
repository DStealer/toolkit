package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var (
	k8sCmd = &cobra.Command{
		Use:   "k8s subcommand [args]",
		Short: "k8s辅助命令",
	}
)

func init() {
	sshdCmd := &cobra.Command{
		Use:   "ssh [args]",
		Short: "ssh网络分析工具",
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

			config, err := clientcmd.BuildConfigFromFlags("", "/home/dstealer/.kube/config")
			cobra.CheckErr(err)
			clientSet := kubernetes.NewForConfigOrDie(config)
			pod, err := clientSet.CoreV1().Pods("default").Get(context.TODO(), "netshoot", metav1.GetOptions{})

			if errors.IsNotFound(err) {
				pod = &v1.Pod{
					TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "netshoot",
						Namespace: "default",
						Labels:    map[string]string{"app": "netshoot", "tier": "devops"},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{{
							Name:            "app",
							Image:           "registry.coded.com/library/netshoot-sshd:latest",
							ImagePullPolicy: v1.PullIfNotPresent,
							Ports: []v1.ContainerPort{
								{Name: "sshd", ContainerPort: 22, Protocol: v1.ProtocolTCP},
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("sshd"),
									},
								},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("sshd"),
									},
								},
							},
							StartupProbe: &v1.Probe{
								Handler: v1.Handler{
									TCPSocket: &v1.TCPSocketAction{
										Port: intstr.FromString("sshd"),
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
				_, err := clientSet.CoreV1().Pods(pod.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
				cobra.CheckErr(err)

				err = wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {
					newPod, err := clientSet.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
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
				})
				cobra.CheckErr(err)

				pod, err = clientSet.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
				cobra.CheckErr(err)
			} else {
				cobra.CheckErr(err)
			}

			if pod.Status.Phase != v1.PodRunning {
				cobra.CheckErr(fmt.Sprintf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase))
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
			fw, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{"22622:22"}, stopChannel, readyChannel, os.Stdout, os.Stderr)
			cobra.CheckErr(err)
			log.Printf("ssh port listen on 127.0.0.1:%v \n", 22622)
			err = fw.ForwardPorts()
			cobra.CheckErr(err)
		},
	}
	k8sCmd.AddCommand(sshdCmd)
}
