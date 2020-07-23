package k8stest

import (
	"errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockKubernetesClient struct {
	Counter map[string]int64
	Nodes   map[string]v1.Node
	Pods    map[string]v1.Pod
}

func NewMockKubernetesClient(nodes []v1.Node, pods []v1.Pod) *MockKubernetesClient {
	client := &MockKubernetesClient{
		Counter: make(map[string]int64),
		Nodes:   make(map[string]v1.Node),
		Pods:    make(map[string]v1.Pod),
	}
	for _, node := range nodes {
		client.Nodes[node.Name] = node
	}
	for _, pod := range pods {
		client.Pods[pod.Name] = pod
	}
	return client
}

func (mock *MockKubernetesClient) GetNodes() ([]v1.Node, error) {
	mock.Counter["GetNodes"]++
	var nodes []v1.Node
	for _, node := range mock.Nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (mock *MockKubernetesClient) GetPodsInNode(node string) ([]v1.Pod, error) {
	mock.Counter["GetPodsInNode"]++
	var pods []v1.Pod
	for _, pod := range mock.Pods {
		if pod.Spec.NodeName == node {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (mock *MockKubernetesClient) GetNodeByAwsInstanceId(awsInstanceId string) (*v1.Node, error) {
	mock.Counter["GetNodeByAwsInstanceId"]++
	for _, node := range mock.Nodes {
		// For the sake of simplicity, we'll just assume that the host name is the same as the node name
		if node.Name == awsInstanceId {
			return &node, nil
		}
	}
	return nil, errors.New("not found")
}

func (mock *MockKubernetesClient) UpdateNode(node *v1.Node) error {
	mock.Counter["UpdateNode"]++
	mock.Nodes[node.Name] = *node
	return nil
}

func (mock *MockKubernetesClient) Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error {
	mock.Counter["Drain"]++
	return nil
}

func CreateTestNode(name string, allocatableCpu, allocatableMemory string) v1.Node {
	node := v1.Node{
		Spec: v1.NodeSpec{},
		Status: v1.NodeStatus{
			Allocatable: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(allocatableCpu),
				v1.ResourceMemory: resource.MustParse(allocatableMemory),
			},
		},
	}
	node.SetName(name)
	node.SetAnnotations(make(map[string]string))
	return node
}

func CreateTestPod(name, nodeName, cpuRequest, cpuMemory string, isDaemonSet bool) v1.Pod {
	pod := v1.Pod{
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Containers: []v1.Container{{
				Name: name,
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse(cpuRequest),
						v1.ResourceMemory: resource.MustParse(cpuMemory),
					},
				},
			}},
		},
	}
	pod.SetName(name)
	if isDaemonSet {
		pod.SetOwnerReferences([]metav1.OwnerReference{{Kind: "DaemonSet"}})
	} else {
		pod.SetOwnerReferences([]metav1.OwnerReference{{Kind: "ReplicaSet"}})
	}
	return pod
}
