package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/inf.v0"

	"github.com/olekukonko/tablewriter"

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
	watch := flag.Bool("watch", false, "(optional) watch at 15 sec intervals")
	namespaceFlag := flag.String("namespace", DefaultNamespace, "(optional) get resources in particular namespace")
	flag.Parse()

	namespace := *namespaceFlag

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
			getNodeStatuses(client, metricClient, namespace)
			time.Sleep(time.Second * 15)
		}
	} else {
		getNodeStatuses(client, metricClient, namespace)
	}
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
			podCount := 0

			for _, pod := range pods.Items {
				if pod.Spec.NodeName == node.Name {
					podCount++
				}
			}
			data = append(data, []string{node.Name, asString(cpuUsage), asStringD(cpuPer), asString(memoryUsage), asStringD(memoryPer), strconv.Itoa(podCount)})
		}
	}
	outputData(data)
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

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func outputData(data [][]string) {
	fmt.Printf("Kubernetes Stats at: %s\n", time.Now())
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %", "Pod Count"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}
