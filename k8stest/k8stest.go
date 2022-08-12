package k8stest

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: replace this by Kubernetes' official fake client (k8s.io/client-go/kubernetes/fake)

type MockClient struct {
	Counter map[string]int64
	Nodes   map[string]v1.Node
	Pods    map[string]v1.Pod
}

func NewMockClient(nodes []v1.Node, pods []v1.Pod) *MockClient {
	client := &MockClient{
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

func (mock *MockClient) GetNodes() ([]v1.Node, error) {
	mock.Counter["GetNodes"]++
	var nodes []v1.Node
	for _, node := range mock.Nodes {
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (mock *MockClient) GetPodsInNode(node string) ([]v1.Pod, error) {
	mock.Counter["GetPodsInNode"]++
	var pods []v1.Pod
	for _, pod := range mock.Pods {
		if pod.Spec.NodeName == node {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (mock *MockClient) GetNodeByAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error) {
	mock.Counter["GetNodeByAutoScalingInstance"]++
	nodes, _ := mock.GetNodes()
	return mock.FilterNodeByAutoScalingInstance(nodes, instance)
}

func (mock *MockClient) FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error) {
	mock.Counter["FilterNodeByAutoScalingInstance"]++
	for _, node := range nodes {
		if node.Spec.ProviderID == fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.AvailabilityZone), aws.StringValue(instance.InstanceId)) {
			return &node, nil
		}
	}
	return nil, errors.New("not found")
}

func (mock *MockClient) UpdateNode(node *v1.Node) error {
	mock.Counter["UpdateNode"]++
	mock.Nodes[node.Name] = *node
	return nil
}

func (mock *MockClient) Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error {
	mock.Counter["Drain"]++
	return nil
}

func CreateTestNode(name, availabilityZone, instanceId, allocatableCpu, allocatableMemory string) v1.Node {
	node := v1.Node{
		Spec: v1.NodeSpec{
			ProviderID: fmt.Sprintf("aws:///%s/%s", availabilityZone, instanceId),
		},
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

func CreateTestPod(name, nodeName, cpuRequest, cpuMemory string, isDaemonSet bool, podPhase v1.PodPhase) v1.Pod {
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
		Status: v1.PodStatus{Phase: podPhase},
	}
	pod.SetName(name)
	if isDaemonSet {
		pod.SetOwnerReferences([]metav1.OwnerReference{{Kind: "DaemonSet"}})
	} else {
		pod.SetOwnerReferences([]metav1.OwnerReference{{Kind: "ReplicaSet"}})
	}
	return pod
}
