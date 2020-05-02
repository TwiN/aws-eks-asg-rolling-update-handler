package k8s

import (
	"errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type MockKubernetesClient struct {
	counter map[string]int64
	nodes   []v1.Node
	pods    []v1.Pod
}

func NewMockKubernetesClient(nodes []v1.Node, pods []v1.Pod) *MockKubernetesClient {
	return &MockKubernetesClient{
		counter: make(map[string]int64),
		nodes:   nodes,
		pods:    pods,
	}
}

func (mock *MockKubernetesClient) GetNodes() ([]v1.Node, error) {
	mock.counter["GetNodes"]++
	return mock.nodes, nil
}

func (mock *MockKubernetesClient) GetPodsInNode(node string) ([]v1.Pod, error) {
	mock.counter["GetPodsInNode"]++
	var pods []v1.Pod
	for _, pod := range mock.pods {
		if pod.Spec.NodeName == node {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (mock *MockKubernetesClient) GetNodeByHostName(hostName string) (*v1.Node, error) {
	mock.counter["GetNodeByHostName"]++
	for _, node := range mock.nodes {
		// For the sake of simplicity, we'll just assume that the host name is the same as the node name
		if node.Name == hostName {
			return &node, nil
		}
	}
	return nil, errors.New("not found")
}

func (mock *MockKubernetesClient) UpdateNode(node *v1.Node) error {
	mock.counter["UpdateNode"]++
	return nil
}

func (mock *MockKubernetesClient) Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error {
	mock.counter["Drain"]++
	return nil
}

func createTestNode(name string, allocatableCpu, allocatableMemory string) v1.Node {
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
	return node
}

func createTestPod(name string, nodeName, cpuRequest, cpuMemory string) v1.Pod {
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
	return pod
}
