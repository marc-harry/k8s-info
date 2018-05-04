package main

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsv1alpha1api "k8s.io/metrics/pkg/apis/metrics/v1alpha1"
)

// Constants
const (
	DefaultHeapsterNamespace = "kube-system"
	DefaultHeapsterScheme    = "http"
	DefaultHeapsterService   = "heapster"
	DefaultHeapsterPort      = "" // use the first exposed port on the service
)

var (
	prefix       = "/apis"
	groupVersion = fmt.Sprintf("%s/%s", metricsGv.Group, metricsGv.Version)
	metricsRoot  = fmt.Sprintf("%s/%s", prefix, groupVersion)

	// TODO: get this from metrics api once it's finished
	metricsGv = schema.GroupVersion{Group: "metrics", Version: "v1alpha1"}
)

// HeapsterMetricsClient heapster client definition
type HeapsterMetricsClient struct {
	SVCClient         corev1.ServicesGetter
	HeapsterNamespace string
	HeapsterScheme    string
	HeapsterService   string
	HeapsterPort      string
}

// NewHeapsterMetricsClient get client with custom heapster settings
func NewHeapsterMetricsClient(svcClient corev1.ServicesGetter, namespace, scheme, service, port string) *HeapsterMetricsClient {
	return &HeapsterMetricsClient{
		SVCClient:         svcClient,
		HeapsterNamespace: namespace,
		HeapsterScheme:    scheme,
		HeapsterService:   service,
		HeapsterPort:      port,
	}
}

// DefaultHeapsterMetricsClient get client with default heapster settings
func DefaultHeapsterMetricsClient(svcClient corev1.ServicesGetter) *HeapsterMetricsClient {
	return NewHeapsterMetricsClient(svcClient, DefaultHeapsterNamespace, DefaultHeapsterScheme, DefaultHeapsterService, DefaultHeapsterPort)
}

func podMetricsURL(namespace string, name string) (string, error) {
	if namespace == metav1.NamespaceAll {
		return fmt.Sprintf("%s/pods", metricsRoot), nil
	}
	return fmt.Sprintf("%s/namespaces/%s/pods/%s", metricsRoot, namespace, name), nil
}

func nodeMetricsURL(name string) (string, error) {
	return fmt.Sprintf("%s/nodes/%s", metricsRoot, name), nil
}

// GetNodeMetrics gets the metrics for a node
func (cli *HeapsterMetricsClient) GetNodeMetrics(nodeName string, selector string) (*metricsapi.NodeMetricsList, error) {
	params := map[string]string{"labelSelector": selector}
	path, err := nodeMetricsURL(nodeName)
	if err != nil {
		return nil, err
	}
	resultRaw, err := GetHeapsterMetrics(cli, path, params)
	if err != nil {
		return nil, err
	}
	versionedMetrics := metricsv1alpha1api.NodeMetricsList{}
	if len(nodeName) == 0 {
		err = json.Unmarshal(resultRaw, &versionedMetrics)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall heapster response: %v", err)
		}
	} else {
		var singleMetric metricsv1alpha1api.NodeMetrics
		err = json.Unmarshal(resultRaw, &singleMetric)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall heapster response: %v", err)
		}
		versionedMetrics.Items = []metricsv1alpha1api.NodeMetrics{singleMetric}
	}
	metrics := &metricsapi.NodeMetricsList{}
	err = metricsv1alpha1api.Convert_v1alpha1_NodeMetricsList_To_metrics_NodeMetricsList(&versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

// GetPodMetrics gets the metrics for a pod
func (cli *HeapsterMetricsClient) GetPodMetrics(namespace string, podName string, allNamespaces bool, selector labels.Selector) (*metricsapi.PodMetricsList, error) {
	if allNamespaces {
		namespace = metav1.NamespaceAll
	}
	path, err := podMetricsURL(namespace, podName)
	if err != nil {
		return nil, err
	}

	params := map[string]string{"labelSelector": selector.String()}
	versionedMetrics := metricsv1alpha1api.PodMetricsList{}

	resultRaw, err := GetHeapsterMetrics(cli, path, params)
	if err != nil {
		return nil, err
	}
	if len(podName) == 0 {
		err = json.Unmarshal(resultRaw, &versionedMetrics)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall heapster response: %v", err)
		}
	} else {
		var singleMetric metricsv1alpha1api.PodMetrics
		err = json.Unmarshal(resultRaw, &singleMetric)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall heapster response: %v", err)
		}
		versionedMetrics.Items = []metricsv1alpha1api.PodMetrics{singleMetric}
	}
	metrics := &metricsapi.PodMetricsList{}
	err = metricsv1alpha1api.Convert_v1alpha1_PodMetricsList_To_metrics_PodMetricsList(&versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

// GetHeapsterMetrics from Heapster service on cluster
func GetHeapsterMetrics(cli *HeapsterMetricsClient, path string, params map[string]string) ([]byte, error) {
	return cli.SVCClient.Services(cli.HeapsterNamespace).
		ProxyGet(cli.HeapsterScheme, cli.HeapsterService, cli.HeapsterPort, path, params).
		DoRaw()
}
