package cmd

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ZZMarquis/gm/sm4"
	"github.com/containerd/containerd"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/prometheus/common/log"
	"github.com/robfig/cron"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestContainerd01(t *testing.T) {
	client, err := containerd.New("/run/containerd/containerd.sock")
	cobra.CheckErr(err)
	defer client.Close()
	list, err := client.NamespaceService().List(context.Background())
	cobra.CheckErr(err)
	fmt.Println(list)
}

func TestK8s01(t *testing.T) {
	client, err := kubernetes.NewForConfig(nil)
	cobra.CheckErr(err)
	slices := client.DiscoveryV1beta1().EndpointSlices("")
	fmt.Print(slices)
}

func TestSm4_1(t *testing.T) {
	key := []byte("J60h6nL19mMZEuDl")
	ptx := []byte("123456789ABC")
	padtext := bytes.Repeat([]byte{byte(0x00)}, 16-len(ptx)%16)
	pptx := append(ptx, padtext...)
	fmt.Printf("填充方式后:%x %d\n", pptx, len(pptx))
	encrypt, err := sm4.ECBEncrypt(key, pptx)
	if err != nil {
		t.Error("加密失败", err)
	}
	fmt.Printf("加密后:%x %d\n", encrypt, len(encrypt))
}
func TestSm4_2(t *testing.T) {
	key := []byte("J60h6nL19mMZEuDl")
	ptx, err := hex.DecodeString("e373f36f37cf9d34fe0094257c0bc433")
	fmt.Printf("解密前:%x %d\n", ptx, len(ptx))
	if err != nil {
		t.Error("解密失败", err)
	}
	encrypt, err := sm4.ECBDecrypt(key, ptx)
	if err != nil {
		t.Error("解密失败", err)
	}
	fmt.Printf("%x %d\n", encrypt, len(encrypt))
	fmt.Printf("密文是:%s\n", strings.TrimFunc(string(encrypt), func(r rune) bool { return r == 0x00 }))
}

func TestCh(t *testing.T) {
	messages := make(chan int, 10)
	done := make(chan struct{})
	defer close(messages)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			select {
			case <-done:
				fmt.Println("child process interrupt...")
				return
			default:
				fmt.Printf("send message: %d\n", <-messages)
			}
		}
	}()
	for i := 0; i < 10; i++ {
		messages <- i
	}
	time.Sleep(10 * time.Second)
	close(done)
	time.Sleep(1 * time.Second)
	fmt.Println("main process exit!")
}

func TestTime(t *testing.T) {
	// #Mon Dec 28 08:40:35 CST 2020
	parse, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", "Mon Dec 28 08:40:35 CST 2020")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(parse)
}

func TestLibDep(t *testing.T) {
	cmd, args, err := jarCmd.Traverse([]string{"dep",
		"Workspaces/JavaProjects/user/api-web/target/"})
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestLibVersion(t *testing.T) {
	cmd, args, err := jarCmd.Traverse([]string{"version",
		"Workspaces/JavaProjects/user/api-web/target"})
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestLibUse(t *testing.T) {
	cmd, args, err := jarCmd.Traverse([]string{"use",
		"Workspaces/JavaProjects/user/api-web/target", "coderd"})
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestVersion(t *testing.T) {
	v1 := "1.1.090-SNAPSHOT"
	v2 := "1.1.019-SNAPSHOT"

	fmt.Println(VersionCompare(v1, v2))
}
func TestGlob(t *testing.T) {
	mt, _ := filepath.Glob("/usr/**/*")
	for _, path := range mt {
		stat, _ := os.Stat(path)
		stat.IsDir()
	}
}
func TestTruncate(t *testing.T) {
	cmd, args, err := logCmd.Traverse([]string{"truncate",
		"/home/dstealer/Data/Temprory/tmp/www/logs/**/app.log"})
	dryRun = true
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestDelete(t *testing.T) {
	cmd, args, err := logCmd.Traverse([]string{"delete",
		"/home/dstealer/Data/Temprory/tmp/www/logs/**/**"})
	dryRun = true
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestDuration(t *testing.T) {
	c := cron.New()
	err := c.AddFunc("* 0 * * * *", func() {
		fmt.Println("aaa")
	})
	if err != nil {
		t.Error(err)
	}
	c.Start()
	fmt.Println(c.Entries()[0].Next)
	select {}
}

func TestTemplate(t *testing.T) {
	projectData := ProjectData{"api-web-0.0.1-SNAPSHOT"}
	tmp, err := template.ParseFS(fileSystem, "assets/*")
	if err != nil {
		t.Error(err)
	}
	err = tmp.Lookup("service.sh").Execute(os.Stdout, projectData)
	if err != nil {
		t.Error(err)
	}
}

func TestServiceCmd(t *testing.T) {
	cmd, args, err := jarCmd.Traverse([]string{"shellgen",
		"Workspaces/JavaProjects/api-web.jar"})
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestJenkinsCmd(t *testing.T) {
	cmd, args, err := jenkinsCmd.Traverse([]string{"uc"})
	cobra.CheckErr(err)
	cmd.Run(cmd, args)
}

func TestK8s(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dstealer/.kube/config")
	if err != nil {
		t.Error(err)
	}
	config.BearerToken = ""
	fmt.Println(config)
	clientSet := kubernetes.NewForConfigOrDie(config)

	podList, err := clientSet.CoreV1().Pods(v1.NamespaceDefault).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Error(err)
	}
	fmt.Println(podList)
}

func TestIndexer(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dstealer/.kube/config")
	if err != nil {
		t.Error(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(config)

	stopCh := make(chan struct{})
	defer close(stopCh)
	factory := informers.NewSharedInformerFactory(clientSet, 0)

	informer := factory.Core().V1().Pods().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mObj := obj.(*v1.Pod)
			log.Infof("new pod: %s", mObj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oObj := oldObj.(*v1.Pod)
			nObj := newObj.(*v1.Pod)
			log.Infof("%v change to: %v", oObj, nObj)
		},
		DeleteFunc: func(obj interface{}) {
			mObj := obj.(*v1.Pod)
			log.Infof("delete pod: %v", mObj)
		},
	})
	informer.Run(stopCh)
}
func TestRegistry(t *testing.T) {
	url := "http://127.0.0.1:5000/"
	username := "" // anonymous
	password := "" // anonymous
	hub, err := registry.New(url, username, password)
	cobra.CheckErr(err)
	repositories, err := hub.Repositories()
	cobra.CheckErr(err)
	for _, repo := range repositories {
		tags, err := hub.Tags(repo)
		cobra.CheckErr(err)
		if !strings.HasPrefix(repo, "coderd/") {
			continue
		}
		fmt.Println(repo)
		for _, tag := range tags {
			if strings.HasSuffix(tag, "22060901") {
				continue
			}
			manifest, err := hub.ManifestV2(repo, tag)
			cobra.CheckErr(err)
			err = hub.DeleteManifest(repo, manifest.Config.Digest)
			cobra.CheckErr(err)
		}
	}
}
func TestHttp(t *testing.T) {
	http.HandleFunc("/k8s", func(writer http.ResponseWriter, request *http.Request) {
		resp, err := http.Get("http://127.0.0.1:42455/")
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
			_ = resp.Header.Write(writer)
			_, _ = writer.Write(buf.Bytes())
			return
		}
		body := buf.String()

		body = strings.ReplaceAll(body, "a", "b")

		err = resp.Header.WriteSubset(writer, map[string]bool{
			"Content-Length":    true,
			"Transfer-Encoding": true,
			"Trailer":           true,
		})
		if err != nil {
			log.Error("写入header错误:", err)
			return
		}
		_, err = writer.Write([]byte(body))
		if err != nil {
			log.Error("写入body错误:", err)
			return
		}
	})

	log.Info("服务器启动监听")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return
	}
}

func TestMem(t *testing.T) {
	targetPercent := 0.6
	deltaPercent := 0.1
	go func() {
		var sl []byte
		ticker := time.NewTicker(1 * time.Second)
		for _ = range ticker.C {
			memory, err := mem.VirtualMemory()
			cobra.CheckErr(err)
			fmt.Printf("Total: %v,Used:%v,Available:%v, Free:%v, UsedPercent:%f %%\n",
				memory.Total/1024/1024, memory.Used/1024/1024, memory.Available/1024/1024, memory.Free/1024/1024, memory.UsedPercent)
			currentPercent := memory.UsedPercent / 100.0
			if currentPercent > (targetPercent + deltaPercent) { //高于上限
				sl = make([]byte, 0, 0)
				fmt.Println("减少内存使用")
			} else if currentPercent < (targetPercent - deltaPercent) { //低于下限
				pct := targetPercent - deltaPercent - currentPercent
				pctByte := pct * float64(memory.Total)
				sl = make([]byte, 0, int(pctByte))
				fmt.Println("增加内存使用")
			} else {

			}
			Unused(sl)
		}
	}()
	select {}
}
func TestCPU(t *testing.T) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for _ = range ticker.C {
			percent, err := cpu.Percent(0, false)
			cobra.CheckErr(err)
			fmt.Println(percent)
		}
	}()
	go func() {
		for {

		}
	}()
	go func() {
		for {

		}
	}()
	select {}
}

func TestMem2(t *testing.T) {
	var sl []byte = make([]byte, 0, 1024*1024*10)

	Unused(sl)
}
