package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/inf.v0"

	"github.com/olekukonko/tablewriter"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/apis/metrics"
)

func main() {
	home := homeDir()
	kubeconfig := flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	watch := flag.Bool("watch", false, "(optional) watch at 15 sec intervals")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	m := &metrics.NodeMetricsList{}
	if m != nil {

	}

	metricClient := DefaultHeapsterMetricsClient(client.CoreV1())

	if *watch {
		for {
			getNodeStatuses(client, metricClient)
			time.Sleep(time.Second * 15)
		}
	} else {
		getNodeStatuses(client, metricClient)
	}
}

func getNodeStatuses(client *kubernetes.Clientset, metricClient *HeapsterMetricsClient) {
	nodes, err := client.CoreV1().Nodes().List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	data := [][]string{}
	for _, node := range nodes.Items {
		metrics, err := metricClient.GetNodeMetrics(node.Name, "String")
		if err != nil {
			panic(err)
		}
		for _, metric := range metrics.Items {
			memoryUsage := metric.Usage.Memory()
			allocMemory := node.Status.Allocatable.Memory()
			cpuUsage := metric.Usage.Cpu()
			allocCPU := node.Status.Allocatable.Cpu()
			memoryPer := getPercentage(memoryUsage, allocMemory)
			cpuPer := getPercentage(cpuUsage, allocCPU)
			data = append(data, []string{node.Name, asString(cpuUsage), asStringD(cpuPer), asString(memoryUsage), asStringD(memoryPer)})
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
	table.SetHeader([]string{"Node", "CPU Usage", "CPU %", "Mem Usage", "Mem %"})
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}