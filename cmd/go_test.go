package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ZZMarquis/gm/sm4"
	auth "github.com/abbot/go-http-auth"
	"github.com/containerd/containerd"
	mysqlclient "github.com/go-mysql-org/go-mysql/client"
	_ "github.com/go-mysql-org/go-mysql/driver"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/go-mysql-org/go-mysql/server"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/pingcap/tidb/parser"
	_ "github.com/pingcap/tidb/parser/test_driver"
	"github.com/prometheus/common/log"
	"github.com/robfig/cron"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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

// ssh端口转发
func TestPortForward(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dstealer/.kube/config")

	if err != nil {
		t.Error(err)
	}
	clientSet := kubernetes.NewForConfigOrDie(config)

	stopChannel := make(chan struct{}, 1)
	readyChannel := make(chan struct{})

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	go func() {
		<-signals
		if stopChannel != nil {
			close(stopChannel)
		}
	}()

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
					Image:           "registry.develop.com:5000/dstealer/netshoot-sshd:latest",
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
		_, err := clientSet.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
		if err != nil {
			t.Error(err)
		}
		err = wait.PollImmediate(1*time.Second, 5*time.Minute, func() (bool, error) {

			pod, err := clientSet.CoreV1().Pods("default").Get(context.TODO(), "netshoot", metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting Pod :%q [%v]\n", "netshoot", err)
				return false, nil
			}
			if pod == nil {
				return false, nil
			}
			if pod.Status.Phase != v1.PodRunning {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			t.Error(err)
		}
		pod, err = clientSet.CoreV1().Pods("default").Get(context.TODO(), "netshoot", metav1.GetOptions{})
		if err != nil {
			t.Error(err)
		}
	}

	if pod.Status.Phase != v1.PodRunning {
		t.Error(fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase))
	}

	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		t.Error(err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	fw, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, []string{"22622:22"}, stopChannel, readyChannel, os.Stdout, os.Stderr)

	if err != nil {
		t.Error(err)
	}
	err = fw.ForwardPorts()
	if err != nil {
		t.Error(err)
	}
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

func TestMem(t *testing.T) {
	targetPercent := 0.6
	deltaPercent := 0.1
	go func() {
		var sl []byte
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			memory, err := mem.VirtualMemory()
			cobra.CheckErr(err)
			fmt.Printf("Total: %v,Used:%v,Available:%v, Free:%v, UsedPercent:%f %%\n", memory.Total/1024/1024, memory.Used/1024/1024, memory.Available/1024/1024, memory.Free/1024/1024, memory.UsedPercent)
			currentPercent := memory.UsedPercent / 100.0
			if currentPercent > (targetPercent + deltaPercent) { //高于上限
				sl = make([]byte, 0, 0)
				fmt.Println("减少内存使用")
			} else if currentPercent < (targetPercent - deltaPercent) { //低于下限
				pct := targetPercent - currentPercent - deltaPercent*rand.Float64()
				pctByte := pct * float64(memory.Total)
				sl = make([]byte, 0, int(pctByte))
				fmt.Println("增加内存使用")
			} else {

			}
			Unused(sl)
		}
		return
	}()
	select {}
}
func TestCPU(t *testing.T) {
	targetPercent := 0.6

	deltaPercent := 0.1

	runtime.GOMAXPROCS(runtime.NumCPU())
	physicalCounts, err := cpu.Counts(false)
	cobra.CheckErr(err)
	Unused(physicalCounts)
	logicalCounts, err := cpu.Counts(true)
	cobra.CheckErr(err)
	totalCounts := logicalCounts * 1000
	go func() {
		for {
			startedTime := time.Now().UnixMilli()

			percent, err := cpu.Percent(0, false)
			cobra.CheckErr(err)
			currentPercent := percent[0] / 100.0

			if currentPercent < targetPercent-deltaPercent {
				averageDeltaCounts := int64((targetPercent-deltaPercent*rand.Float64()-currentPercent)*float64(totalCounts)) / int64(logicalCounts)
				fmt.Println("averageDeltaCounts:", averageDeltaCounts)
				for i := 0; i < logicalCounts; i++ {
					go func() {
						startedTime := time.Now().UnixMilli()
						for (time.Now().UnixMilli() - startedTime) < averageDeltaCounts {
						}
						sleepMills := 1000 - (time.Now().UnixMilli() - startedTime)
						if sleepMills <= 0 {
							time.Sleep(0)
						} else {
							time.Sleep(time.Duration(sleepMills) * time.Millisecond)
						}
						return
					}()
				}
			}
			sleepMills := 1000 - (time.Now().UnixMilli() - startedTime)
			fmt.Println("sleep:", sleepMills)
			if sleepMills <= 0 {
				time.Sleep(0 * time.Millisecond)
			} else {
				time.Sleep(time.Duration(sleepMills) * time.Millisecond)
			}
		}
		return
	}()
	select {}
}

func TestMysql(t *testing.T) {
	dsn := "root@127.0.0.1:3306?test"
	db, _ := sql.Open("mysql", dsn)
	err := db.Ping()
	cobra.CheckErr(err)
	db.Close()
}
func TestMysql2(t *testing.T) {
	config := replication.BinlogSyncerConfig{
		ServerID: 100,
		Flavor:   "mysql",
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
	}
	syncer := replication.NewBinlogSyncer(config)
	sync, err := syncer.StartSync(mysql.Position{})
	cobra.CheckErr(err)
	event, err := sync.GetEvent(context.Background())
	cobra.CheckErr(err)
	event.Dump(os.Stdout)
}
func TestMysql3(t *testing.T) {
	conn, err := mysqlclient.Connect("192.168.122.1:3306", "root", "root@123", "")
	cobra.CheckErr(err)
	defer conn.Close()
	result, err := conn.Execute("select * from library.user")
	cobra.CheckErr(err)
	defer result.Close()
	for _, row := range result.Values {
		vals := make([]interface{}, len(result.Fields))
		for index, val := range row {
			if val.Type == mysql.FieldValueTypeString {
				vals[index] = string(val.AsString())
			} else if val.Type == mysql.FieldValueTypeNull {
				vals[index] = nil
			} else {
				vals[index] = val.Value()
			}
		}
		fmt.Println("data :", vals)
	}
}

func TestMysql4(t *testing.T) {
	conn, err := mysqlclient.Connect("192.168.122.1:3306", "root", "root@123", "library")
	table := "user"
	where := "1=1"

	cobra.CheckErr(err)
	defer conn.Close()
	var result mysql.Result
	defer result.Close()

	err = conn.ExecuteSelectStreaming(fmt.Sprintf("SELECT * FROM `%s` WHERE %s ;", table, where), &result, func(row []mysql.FieldValue) error {
		names := make([]string, len(result.Fields))
		values := make([]string, len(result.Fields))
		for index, val := range row {
			if val.Type == mysql.FieldValueTypeString {
				values[index] = fmt.Sprintf("'%s'", string(val.AsString()))
			} else if val.Type == mysql.FieldValueTypeNull {
				values[index] = "NULL"
			} else {
				values[index] = fmt.Sprintf("%v", val.Value())
			}
			names[index] = fmt.Sprintf("`%s`", string(result.Fields[index].Name))
		}
		fmt.Printf("INSERT INTO `%s` (%s) VALUES (%s);\n", table, strings.Join(names, ","), strings.Join(values, ","))
		return nil
	}, func(result *mysql.Result) error {
		return nil
	})
	cobra.CheckErr(err)
}
func Secret(user, realm string) string {
	if user == "john" {
		// password is "hello"
		return "$1$dlPL2MqE$oQmn16q49SqdmhenQuNgs1"
	}
	return ""
}

func TestHttpAuth(t *testing.T) {
	provider := auth.HtpasswdFileProvider("/data/Temprory/htpasswd.cfg")

	upstream, err := url.ParseRequestURI("https://www.sina.com")
	cobra.CheckErr(err)

	authenticator := auth.NewBasicAuthenticator(upstream.Hostname(), provider)
	http.HandleFunc("/", authenticator.Wrap(reverseProxy(*upstream)))

	err = http.ListenAndServe("127.0.0.1:8080", nil)
	cobra.CheckErr(err)
}

type HttpTransferHandler struct {
	BaseUrl string
	Client  http.Client
}

func (h HttpTransferHandler) UseDB(dbName string) error {
	return fmt.Errorf("not supported now")
}
func (h HttpTransferHandler) HandleQuery(query string) (*mysql.Result, error) {
	return nil, fmt.Errorf("not supported now")
}

func (h HttpTransferHandler) HandleFieldList(table string, fieldWildcard string) ([]*mysql.Field, error) {
	return nil, fmt.Errorf("not supported now")
}

func (h HttpTransferHandler) HandleStmtPrepare(query string) (int, int, interface{}, error) {
	if !strings.EqualFold("select httpStatus,headers,body from t_transfer where methodParam = ? and uriParam= ? and queryParam= ? and headerParam= ? and bodyParam = ?", query) {
		return 0, 0, nil, fmt.Errorf("not supported stmt now")
	}
	return 5, 3, context.Background(), nil
}

func (h HttpTransferHandler) HandleStmtExecute(ctx interface{}, query string, args []interface{}) (*mysql.Result, error) {
	if len(args) != 5 {
		return nil, fmt.Errorf("args len wrong")
	}
	methodParam := cast.ToString(args[0])
	uriParam := cast.ToString(args[1])
	queryParam := cast.ToString(args[2])
	headerParam := cast.ToString(args[3])
	bodyParam := cast.ToString(args[4])

	var url string
	if h.BaseUrl == "" {
		url = fmt.Sprintf("%s%s?%s", h.BaseUrl, uriParam, queryParam)
	} else {
		url = fmt.Sprintf("%s?%s", uriParam, queryParam)
	}
	request, err := http.NewRequest(methodParam, url, strings.NewReader(bodyParam))
	if headerParam != "" {
		headers := make(map[string][]string, 4)
		err := json.Unmarshal([]byte(headerParam), &headers)
		if err != nil {
			return nil, err
		}
		for name, values := range headers {
			//remove Hop-by-hop header
			if ContainsFold(name, "Host", "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailers", "Transfer-Encoding", "Upgrade") {
				continue
			}
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}
	}
	response, err := h.Client.Do(request)
	if err != nil {
		log.Warnf("request:%s error:%v", url, err)
		return nil, err
	}

	httpStatus := response.StatusCode

	log.Infof("request:%s success :%d", url, httpStatus)

	header := response.Header
	for _, hd := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailers", "Transfer-Encoding", "Upgrade"} {
		header.Del(hd)
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	headerJson := string(headerBytes)

	defer response.Body.Close()

	bytes, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}
	body := string(bytes)

	r, err := mysql.BuildSimpleBinaryResultset([]string{"httpStatus", "headers", "body"}, [][]interface{}{{httpStatus, headerJson, body}})

	if err != nil {
		return nil, err
	}
	return &mysql.Result{
		Status:       0,
		Warnings:     0,
		InsertId:     0,
		AffectedRows: 0,
		Resultset:    r,
	}, nil
}

func (h HttpTransferHandler) HandleStmtClose(ctx interface{}) error {
	return nil
}

func (h HttpTransferHandler) HandleOtherCommand(cmd byte, data []byte) error {
	return fmt.Errorf("not supported operation")
}

func TestSqlProxy(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:3306")
	if err != nil {
		log.Fatal(err)
	}
	client := http.Client{Timeout: 60 * time.Second}
	baseUrl := "https://www.baidu.com"
	defer listener.Close()
	for {
		// Accept a new connection once
		c, err := listener.Accept()
		if err != nil {
			log.Warn(err)
			continue
		}
		conn, err := server.NewConn(c, "root", "root", HttpTransferHandler{
			BaseUrl: baseUrl, Client: client})
		if err != nil {
			log.Warn(err)
			continue
		}
		go func() {
			for {
				if err := conn.HandleCommand(); err != nil {
					log.Warn(err)
					return
				}
			}
			return
		}()
	}
}

func TestQueryDb(t *testing.T) {

	pool := mysqlclient.NewPool(log.Infof, 10, 100, 10, "127.0.0.1:3306", "root", "root", "")

	timeout, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)

	defer cancelFunc()

	conn, err := pool.GetConn(timeout)
	cobra.CheckErr(err)

	defer conn.Close()
	methodParam := "GET"
	uriParam := "/"
	queryParam := "a=b&c=d"
	headerParam := ""
	bodyParam := ""

	result, err := conn.Execute("select httpStatus,headers,body from t_transfer where methodParam = ? and uriParam= ? and queryParam= ? and headerParam= ? and bodyParam = ?", []interface{}{methodParam, uriParam, queryParam, headerParam, bodyParam}...)

	cobra.CheckErr(err)

	defer result.Close()
	rs := result.Resultset
	if rs == nil {
		return
	}
	httpStatus, err := rs.GetIntByName(0, "httpStatus")
	cobra.CheckErr(err)
	headers, err := rs.GetStringByName(0, "headers")
	cobra.CheckErr(err)
	body, err := rs.GetStringByName(0, "body")
	cobra.CheckErr(err)
	log.Infof("响应数据:%d ,%s ,%s ", httpStatus, headers, body)
}

func TestSqlParser(t *testing.T) {
	p := parser.New()

	stmtNodes, _, err := p.Parse("SELECT a, b FROM t", "utf8", "")
	cobra.CheckErr(err)

	log.Infof("data:%v", &stmtNodes[0])
}
