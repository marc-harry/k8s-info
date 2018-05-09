package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	typesv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// Default Constants
const (
	DefaultNamespace = "default"
)

// KubeInfoService basic information service
type KubeInfoService struct {
	Client        corev1.CoreV1Interface
	MetricClient  *HeapsterMetricsClient
	Namespace     string
	AllNamespaces bool
	Metric        string
}

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

	service := &KubeInfoService{client.CoreV1(), metricClient, namespace, *all, *metric}
	if *watch {
		for {
			processRequest(service)
			time.Sleep(time.Second * time.Duration(durationSeconds))
		}
	} else {
		processRequest(service)
	}
}

func processRequest(service *KubeInfoService) {
	switch service.Metric {
	case "nodes":
		getNodeStatuses(service)
	case "pods":
		getPodStatuses(service)
	default:
		fmt.Println("Invalid metric supplied.")
		os.Exit(1)
	}
}

func getPodStatuses(service *KubeInfoService) {
	pods, err := service.Client.Pods(service.Namespace).List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	data := [][]string{}

	stats := make(chan []string)
	statCount := len(pods.Items)
	for _, pod := range pods.Items {
		go getPodStats(stats, pod, service)
	}

	outputInfo := [][]string{}
	for statCount != len(data) {
		data = append(data, <-stats)
	}
	for _, val := range data {
		if val != nil {
			outputInfo = append(outputInfo, val)
		}
	}
	sort.Sort(byName(outputInfo))
	headers := []string{"Pod", "Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %", "Status", "Up time", "Restarts", "Last Restart"}
	outputData(headers, outputInfo)
}

func getNodeStatuses(service *KubeInfoService) {
	nodes, err := service.Client.Nodes().List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	pods, err := service.Client.Pods(service.Namespace).List(v1.ListOptions{})
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
		metrics, err := service.MetricClient.GetNodeMetrics(node.Name, labels.Everything().String())
		if err != nil {
			fmt.Printf("Failed to get metrics for Node: %s\n", node.Name)
			continue
		}
		nodeState := ""
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" {
				switch condition.Status {
				case "True":
					nodeState = "Ready"
				case "False":
					nodeState = "Not Ready"
				case "Unknown":
					nodeState = "Unknown"
				}
			}
		}

		for _, metric := range metrics.Items {
			memoryUsage := metric.Usage.Memory()
			allocMemory := node.Status.Allocatable.Memory()
			cpuUsage := metric.Usage.Cpu()
			allocCPU := node.Status.Allocatable.Cpu()
			memoryPer := getPercentage(memoryUsage, allocMemory)
			cpuPer := getPercentage(cpuUsage, allocCPU)
			podCount := len(nodePods[node.Name])

			data = append(data, []string{node.Name, asString(cpuUsage), asString(cpuPer), asString(memoryUsage), asString(memoryPer), strconv.Itoa(podCount), nodeState})
		}
	}
	headers := []string{"Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %", "Pod Count", "State"}
	outputData(headers, data)
	if len(failingPods) > 0 {
		outputFailing(failingPods)
	}
}

func getPodStats(stats chan<- []string, pod typesv1.Pod, service *KubeInfoService) {
	metrics, err := service.MetricClient.GetPodMetrics(service.Namespace, pod.Name, service.AllNamespaces, labels.Everything())
	if err != nil {
		fmt.Printf("Failed to get logs for Pod: %s\n", pod.Name)
		stats <- nil
		return
	}
	node, err := service.Client.Nodes().Get(pod.Spec.NodeName, v1.GetOptions{})
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
		restartsCount := int(pod.Status.ContainerStatuses[0].RestartCount)

		upTime := getTimeSince(pod.Status.StartTime.Time)
		lastRestart := ""
		if restartsCount > 0 {
			lastRestart = getTimeSince(pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.FinishedAt.Time)
		}
		stats <- []string{pod.Name, pod.Spec.NodeName, asString(cpuUsage), asString(cpuPer), asString(memoryUsage),
			asString(memoryPer), asString(pod.Status.Phase), upTime, strconv.Itoa(restartsCount), lastRestart}
	}
}
