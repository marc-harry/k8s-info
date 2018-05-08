package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"gopkg.in/inf.v0"

	"github.com/olekukonko/tablewriter"

	typesv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Default Constants
const (
	DefaultNamespace = "default"
)

func main() {
	home := homeDir()
	kubeconfig := flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	watch := flag.Bool("watch", false, "(optional) watch at intervals 15 second by default")
	namespaceFlag := flag.String("namespace", DefaultNamespace, "(optional) get resources in particular namespace")
	duration := flag.Int("duration", 15, "(optional) set watch interval to custom duration in seconds")
	all := flag.Bool("all", false, "(optional) get all namespaces (this will override --namespace)")
	metric := flag.String("metric", "nodes", "(required) Metric {nodes|pods};.")
	flag.Parse()

	namespace := *namespaceFlag
	durationSeconds := *duration

	if *all {
		namespace = ""
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	metricClient := DefaultHeapsterMetricsClient(client.CoreV1())

	if *watch {
		for {
			processRequest(client, metricClient, namespace, *metric)
			time.Sleep(time.Second * time.Duration(durationSeconds))
		}
	} else {
		processRequest(client, metricClient, namespace, *metric)
	}
}

func processRequest(client *kubernetes.Clientset, metricClient *HeapsterMetricsClient, namespace string, metric string) {
	switch metric {
	case "nodes":
		getNodeStatuses(client, metricClient, namespace)
	case "pods":
		getPodStatuses(client, metricClient, namespace)
	default:
		fmt.Println("Invalid metric supplied.")
		os.Exit(1)
	}
}

func getPodStatuses(client *kubernetes.Clientset, metricClient *HeapsterMetricsClient, namespace string) {
	pods, err := client.CoreV1().Pods(namespace).List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	data := [][]string{}

	stats := make(chan []string)
	statCount := len(pods.Items)
	for _, pod := range pods.Items {
		go getPodStats(stats, client, metricClient, namespace, pod)
	}

	for statCount != len(data) {
		data = append(data, <-stats)
	}
	sort.Sort(byName(data))
	outputPodData(data)
}

func getNodeStatuses(client *kubernetes.Clientset, metricClient *HeapsterMetricsClient, namespace string) {
	nodes, err := client.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	pods, err := client.CoreV1().Pods(namespace).List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	nodePods := map[string][]string{}
	failingPods := map[string]typesv1.PodPhase{}
	for _, pod := range pods.Items {
		nodePods[pod.Spec.NodeName] = append(nodePods[pod.Spec.NodeName], pod.Name)
		if pod.Status.Phase != typesv1.PodRunning {
			failingPods[pod.Name] = pod.Status.Phase
		}
	}
	data := [][]string{}
	for _, node := range nodes.Items {
		metrics, err := metricClient.GetNodeMetrics(node.Name, labels.Everything().String())
		if err != nil {
			fmt.Printf("Failed to get metrics for Node: %s\n", node.Name)
			continue
		}
		for _, metric := range metrics.Items {
			memoryUsage := metric.Usage.Memory()
			allocMemory := node.Status.Allocatable.Memory()
			cpuUsage := metric.Usage.Cpu()
			allocCPU := node.Status.Allocatable.Cpu()
			memoryPer := getPercentage(memoryUsage, allocMemory)
			cpuPer := getPercentage(cpuUsage, allocCPU)
			podCount := len(nodePods[node.Name])

			data = append(data, []string{node.Name, asString(cpuUsage), asStringD(cpuPer), asString(memoryUsage), asStringD(memoryPer), strconv.Itoa(podCount)})
		}
	}
	outputNodeData(data)
	if len(failingPods) > 0 {
		outputFailing(failingPods)
	}
}

func getPercentage(first *resource.Quantity, second *resource.Quantity) *inf.Dec {
	val := new(inf.Dec).QuoRound(first.AsDec(), second.AsDec(), 2, inf.RoundCeil)
	per := new(inf.Dec).Mul(val, inf.NewDec(100, 0))
	return per
}

func asString(res *resource.Quantity) string {
	return fmt.Sprintf("%s", res)
}

func asStringD(res *inf.Dec) string {
	return fmt.Sprintf("%d", res)
}

func asStringP(res typesv1.PodPhase) string {
	return fmt.Sprintf("%s", res)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func outputNodeData(data [][]string) {
	fmt.Printf("Kubernetes Node Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %", "Pod Count"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}

func outputPodData(data [][]string) {
	fmt.Printf("Kubernetes Pod Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pod", "Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}

func outputFailing(dataMap map[string]typesv1.PodPhase) {
	data := [][]string{}
	for podName, podInfo := range dataMap {
		data = append(data, []string{podName, asStringP(podInfo)})
	}
	fmt.Printf("Failing Pod Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pod", "Status"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}

func getPodStats(stats chan<- []string, client *kubernetes.Clientset, metricClient *HeapsterMetricsClient, namespace string, pod typesv1.Pod) {
	metrics, err := metricClient.GetPodMetrics(namespace, pod.Name, false, labels.Everything())
	if err != nil {
		fmt.Printf("Failed to get logs for Pod: %s", pod.Name)
		stats <- nil
	}
	node, err := client.CoreV1().Nodes().Get(pod.Spec.NodeName, v1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, metric := range metrics.Items {
		memoryUsage := metric.Containers[0].Usage.Memory()
		allocMemory := node.Status.Allocatable.Memory()
		cpuUsage := metric.Containers[0].Usage.Cpu()
		allocCPU := node.Status.Allocatable.Cpu()
		memoryPer := getPercentage(memoryUsage, allocMemory)
		cpuPer := getPercentage(cpuUsage, allocCPU)

		stats <- []string{pod.Name, pod.Spec.NodeName, asString(cpuUsage), asStringD(cpuPer), asString(memoryUsage), asStringD(memoryPer)}
	}
}

type byName [][]string

func (s byName) Len() int {
	return len(s)
}
func (s byName) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byName) Less(i, j int) bool {
	return s[i][0] < s[j][0]
}
