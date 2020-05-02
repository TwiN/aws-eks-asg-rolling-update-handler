package k8s

import (
	"fmt"
	drain "github.com/openshift/kubernetes-drain"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

const (
	ScaleDownDisabledAnnotationKey                = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	RollingUpdateStartedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/started-at"
	RollingUpdateDrainedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/drained-at"
	RollingUpdateTerminatedTimestampAnnotationKey = "aws-eks-asg-rolling-update-handler/terminated-at"
	HostNameAnnotationKey                         = "kubernetes.io/hostname"
)

type KubernetesClientApi interface {
	GetNodes() ([]v1.Node, error)
	GetPodsInNode(node string) ([]v1.Pod, error)
	GetNodeByHostName(hostName string) (*v1.Node, error)
	UpdateNode(node *v1.Node) error
	Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error
}

type KubernetesClient struct {
	client *kubernetes.Clientset
}

func NewKubernetesClient(client *kubernetes.Clientset) *KubernetesClient {
	return &KubernetesClient{
		client: client,
	}
}

func (k *KubernetesClient) GetNodes() ([]v1.Node, error) {
	nodeList, err := k.client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (k *KubernetesClient) GetPodsInNode(node string) ([]v1.Pod, error) {
	podList, err := k.client.CoreV1().Pods("").List(metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func (k *KubernetesClient) GetNodeByHostName(hostName string) (*v1.Node, error) {
	api := k.client.CoreV1().Nodes()
	nodeList, err := api.List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", HostNameAnnotationKey, hostName),
		Limit:         1,
	})
	if err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("nodes with hostname \"%s\" not found", hostName)
	}
	return &nodeList.Items[0], nil
}

func (k *KubernetesClient) UpdateNode(node *v1.Node) error {
	api := k.client.CoreV1().Nodes()
	_, err := api.Update(node)
	return err
}

func (k *KubernetesClient) Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error {
	node, err := k.client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// set options and drain nodes
	return drain.Drain(k.client, []*v1.Node{node}, &drain.DrainOptions{
		IgnoreDaemonsets:   ignoreDaemonSets,
		GracePeriodSeconds: -1,
		Force:              true,
		DeleteLocalData:    deleteLocalData,
		Timeout:            3 * time.Minute,
	})
}
