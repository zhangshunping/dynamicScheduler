package main

import (
	"context"
	"dynamicScheduler/prom"
	"dynamicScheduler/utils"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"io"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	FitSelectorAndAlreadyLabelPresureNodes     []*v1.Node //符合标签选择器，
	FitSelectorAndAlreadyLabelPresureNodeNames []string   //集群内已经上压力的node Names
	FitSelectorNodes                           []*v1.Node //根据标签选择器获取的当前node slice
	presureNodesNameFromProm                   []string
	count, scrape_interval                    int
	promAddress,webaddr,rulepath                               string
	UpperPresureNodeNames []string
)

const (
	CgroupsMemOpen =true
	CgroupsMemClose =false
)
type Response struct {
	Type      string   `json:"type"`
	Num       int      `json:"num"`
	NodeNames []string `json:"node_names"`

}

func init() {
	flag.IntVar(&scrape_interval, "s", 10, "每次抓取prometheus metrics间隔（-s 10)")
	flag.StringVar(&promAddress, "prom", "http://121.40.XX.XX:49090", "prometheus链接地址(-prom http://121.40.XX.XX:49090)")
	flag.StringVar(&webaddr, "webaddr", ":9000", "启动服务端口地址(-webaddr :9000)")
	flag.StringVar(&rulepath,"r","rule.yaml" , "查询prometheus 阈值规则")
}

func main() {
	ctx := context.Background()
	stopCh := make(chan struct{})
	//1.k8sClient初始化
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, "src", "config"), "链接k8s kubeconfig的绝对路径")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "链接k8s kubeconfig绝对路径")
	}
	flag.Parse()
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
		os.Exit(0)
	}

	//2.prometheus 客户都初始化
	client, err := api.NewClient(api.Config{
		Address: promAddress,
	})
	if err != nil {
		utils.Log.Error("Error creating client: %v\n", err)
		os.Exit(1)
	}
	v1api := promv1.NewAPI(client)
	//3.k8s NewSharedInfomerFactory
	factory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
	//4.跟据prometheus获取对应的metrics项目，发现超出的阈值的节点，则给其打上type=presure标签
	LabelNodeByPromMetrics(stopCh, factory, ctx, clientset, v1api, scrape_interval)
	// 设置多路复用处理函数, 为了健康检查
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes", ListPresureNodes)
	mux.HandleFunc("/status", Healyth)
	// 设置服务器
	server := &http.Server{
		Addr:    webaddr,
		Handler: mux,
	}
	// 设置服务器监听请求端口
	server.ListenAndServe()

	<-stopCh

}

// 设置多个处理器函数
func ListPresureNodes(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Type:      "Label type=presure Nodes",
		Num:       len(FitSelectorAndAlreadyLabelPresureNodeNames),
		NodeNames: FitSelectorAndAlreadyLabelPresureNodeNames,
	}
	jsonbyte, err := json.Marshal(response)
	if err != nil {
		utils.Log.Error("struct --> json", err)
	}
	io.WriteString(w, string(jsonbyte))

}

func Healyth(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "ok")
}


func LabelNodeByPromMetrics(stopCh <-chan struct{}, factory informers.SharedInformerFactory,
	ctx context.Context, clientset *kubernetes.Clientset, v1api promv1.API, scrape_interval int) {

	//1 node Informer
	nodeInformer := factory.Core().V1().Nodes()
	//1.1开启node informer
	go nodeInformer.Informer().Run(stopCh)
	//1.2从k8s中同步list node
	if !cache.WaitForCacheSync(stopCh, nodeInformer.Informer().HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}
	//2.node selector, 返回打了others和persure节点*[]v1.Node
	//nodeOthersSlice := []string{"others"}
	nodeOthersSlice2 := []string{"presure"}
	//selector, _ := labels.NewRequirement("type", selection.In, nodeOthersSlice)
	selector2, _ := labels.NewRequirement("status", selection.In, nodeOthersSlice2)
	//3.go程循环监听prometheus,根据metrics跟node加上label
	go func() {
		for {
			count = count + 1
			now := time.Now()
			utils.Log.Infof("Staring,id=%v", count)
			//查询所有节点all
			FitSelectorNodes, _ = nodeInformer.Lister().List(labels.NewSelector())
			// 监控使用，查看节点内已经打上presure节点的node
			FitSelectorAndAlreadyLabelPresureNodes, _ = nodeInformer.Lister().List(labels.NewSelector().Add(*selector2))
			FitSelectorAndAlreadyLabelPresureNodeNames = []string{} //清空所有的Nodes
			for i := 0; i < len(FitSelectorAndAlreadyLabelPresureNodes); i++ {
				FitSelectorAndAlreadyLabelPresureNodeNames = append(FitSelectorAndAlreadyLabelPresureNodeNames, FitSelectorAndAlreadyLabelPresureNodes[i].Name)
			}
			//3.1获取超出阈值的node节点 names
			presureNodesNameFromProm = []string{} // 清空presureNodesName nodes
			for k, v := range(utils.GetRuleFromYaml(rulepath)) {
				m,_:=prom.QueryRebuild(v1api, ctx,v["Promsql"],time.Now())
				th:=v["Threshold"]
				floatth,_:=strconv.ParseFloat(th, 64)
				UpperPresureNodeNames = CountUpperPresureNodeFromProm(m, floatth,k,CgroupsMemClose)
				utils.Log.Infof("%s 超过%v的节点列表：%v", k,floatth,UpperPresureNodeNames)
			}

			/* 旧代码逻辑
			//3.2计算过去一分钟cpu的使用率
			utils.Log.Info("============Cpu==========")
			resultFromPromSilceMap, _ := prom.QueryRebuild(v1api, ctx, prom.Node_cpu1, time.Now())
			UpperPresureNodeNames := CountUpperPresureNodeFromProm(resultFromPromSilceMap, cpuThreshold, "cpu",CgroupsMemClose)
			utils.Log.Infof("Cpu使用率超过%v的节点列表：%v", cpuThreshold,UpperPresureNodeNames)
			//3.3计算mem使用率
			utils.Log.Info("============Mem==========")
			resultFromPromSilceMapMem, _ := prom.QueryRebuild(v1api, ctx, prom.Node_mem, time.Now())
			UpperPresureNodeNames = CountUpperPresureNodeFromProm(resultFromPromSilceMapMem, memThreshold, "mem",CgroupsMemClose)
			utils.Log.Infof("Mem 使用率超过阈值%v的节点列表: %v ", memThreshold, UpperPresureNodeNames)
			//计算/sys/fs/cgroup/ 下 memory.avaialbe的值（及kubelet发生驱逐的值）
			CgroupMems,_:=prom.QueryRebuild(v1api,ctx,prom.K8_node_cgroups_mem_available,time.Now())
			UpperPresureNodeNames = CountUpperPresureNodeFromProm(CgroupMems, CgroupsMemThreshold, "CgoupsMemAvailable",CgroupsMemOpen)
			utils.Log.Infof("节点memory.available低于%v阈值的节点列表： %v ", CgroupsMemThreshold, UpperPresureNodeNames)
			utils.Log.Info("============Label==========")
			*/

			//3.2清空v1.Node列表(打赏presure标签的Nodes和去掉标签的Nodes)
			ReadyForLabelPresure := []*v1.Node{}
			ReadyForLabelNil := []*v1.Node{}
			//3.3查询需要节点的nodes
			for _, v := range FitSelectorNodes {
				key := v.Name
				if IsExitArray(key, UpperPresureNodeNames) {
					ReadyForLabelPresure = append(ReadyForLabelPresure, v)
				} else {
					ReadyForLabelNil = append(ReadyForLabelNil, v)
				}
			}
			//3.4 label status=presure 和去掉 status=presure
			PatchNode(clientset, ctx, "presure", ReadyForLabelPresure)
			//3.5需补充，对已经为nil的节点则不重复patch为nil*****-******************
			PatchNode(clientset, ctx, "nil", ReadyForLabelNil)

			utils.Log.Infof("当前有%d个节点已经处于status=presure状态: %v", len(FitSelectorAndAlreadyLabelPresureNodeNames), FitSelectorAndAlreadyLabelPresureNodeNames)
			utils.Log.Infof("Ending,id=%v,耗时%v", count, time.Since(now))
			utils.Log.Info("<-------------------------------------->")

			time.Sleep(time.Second * time.Duration(scrape_interval))
		}
	}()

}

//patch node label
func PatchNode(clientset *kubernetes.Clientset, ctx context.Context, selection string, Nodes []*v1.Node) {
	//const selection pressure or nil
	var labelvaule interface{}
	labelkey := "status"
	switch selection {
	//去掉标签
	case "nil":
		labelvaule = nil
	//打上标签
	case "presure":
		labelvaule = fmt.Sprintf("%s", selection)
	}
	patchTemplate := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				labelkey: labelvaule,
			},
		},
	}
	patchdata, _ := json.Marshal(patchTemplate)
	for i := 0; i < len(Nodes); i++ {
		//master节点不做label处理
		if _, ok := Nodes[i].Labels["node-role.kubernetes.io/master"]; !ok {
			_, err := clientset.CoreV1().Nodes().Patch(ctx, Nodes[i].Name, types.StrategicMergePatchType, patchdata, metav1.PatchOptions{})
			if err == nil {
				utils.Log.Infof("给节点%s打type=%s标签成功", Nodes[i].Name, selection)
			} else {

				utils.Log.Errorf("给节点%s打%s标签失败，错误为%v", Nodes[i].Name, selection, err)
			}
		} else {
			utils.Log.Infof("%s节点为master节点，不参与平衡调度标签策略", Nodes[i].Name)
		}

	}
}

// 根据n阈值，筛选超出阈值node节点（master和slave)
func CountUpperPresureNodeFromProm(resultFromPromSilceMap []map[string]string, threshold float64, metricName string,cgroupsbool bool) []string {
	for i := 0; i < len(resultFromPromSilceMap); i++ {
		currentMetrics, _ := strconv.ParseFloat(resultFromPromSilceMap[i]["value"], 64)
		utils.Log.Infof("%s节点当前%s使用率为: %v", resultFromPromSilceMap[i]["instance"], metricName, currentMetrics)
		// cgroupsMem判断及 kubelet驱逐指标 memory.available的判断
		if cgroupsbool{
			if currentMetrics <threshold{
				if !IsExitArray(resultFromPromSilceMap[i]["instance"], presureNodesNameFromProm) {
					presureNodesNameFromProm = append(presureNodesNameFromProm, resultFromPromSilceMap[i]["instance"])
				}
			}
		}
		// 判断负载值是否
		if currentMetrics > threshold {
			if !IsExitArray(resultFromPromSilceMap[i]["instance"], presureNodesNameFromProm) {
				presureNodesNameFromProm = append(presureNodesNameFromProm, resultFromPromSilceMap[i]["instance"])
			}
		}
	}
	return presureNodesNameFromProm
}

func homeDir() string {
	if h := os.Getenv("GOPATH"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

func IsExitArray(value string, arry []string) bool {
	for _, v := range arry {
		if v == value {
			return true
		}
	}
	return false
}

