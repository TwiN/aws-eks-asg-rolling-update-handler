package k8s

import (
	"fmt"
	drain "github.com/openshift/kubernetes-drain"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"time"
)

const (
	ScaleDownDisabledAnnotationKey                = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	RollingUpdateStartedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/started-at"
	RollingUpdateDrainedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/drained-at"
	RollingUpdateTerminatedTimestampAnnotationKey = "aws-eks-asg-rolling-update-handler/terminated-at"
	HostNameAnnotationKey                         = "kubernetes.io/hostname"
)

func GetNodes() ([]corev1.Node, error) {
	client, err := CreateClient()
	if err != nil {
		return nil, err
	}
	nodeList, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func GetPodsInNode(node string) ([]corev1.Pod, error) {
	client, err := CreateClient()
	podList, err := client.CoreV1().Pods("").List(metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func GetNodeByHostName(hostName string) (*corev1.Node, error) {
	client, err := CreateClient()
	if err != nil {
		return nil, err
	}
	api := client.CoreV1().Nodes()
	nodeList, err := api.List(v1.ListOptions{
		LabelSelector: fmt.Sprintf("kubernetes.io/hostname=%s", hostName),
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

func AnnotateNodeByHostName(hostName, key, value string) error {
	client, err := CreateClient()
	if err != nil {
		return err
	}
	node, err := GetNodeByHostName(hostName)
	if err != nil {
		return err
	}
	annotations := node.GetAnnotations()
	if currentValue := annotations[key]; currentValue != value {
		annotations[key] = value
		node.SetAnnotations(annotations)
		api := client.CoreV1().Nodes()
		_, err := api.Update(node)
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateNode(node *corev1.Node) error {
	client, err := CreateClient()
	if err != nil {
		return err
	}
	api := client.CoreV1().Nodes()
	_, err = api.Update(node)
	return err
}

func Drain(nodeName string, ignoreDaemonSets, deleteLocalData bool) error {
	client, err := CreateClient()
	if err != nil {
		return err
	}
	node, err := client.CoreV1().Nodes().Get(nodeName, v1.GetOptions{})
	if err != nil {
		return err
	}
	// set options and drain nodes
	return drain.Drain(client, []*corev1.Node{node}, &drain.DrainOptions{
		IgnoreDaemonsets:   ignoreDaemonSets,
		GracePeriodSeconds: -1,
		Force:              true,
		DeleteLocalData:    deleteLocalData,
		Timeout:            3 * time.Minute,
	})
}

func CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(oldNode *corev1.Node, targetNodes []*corev1.Node) bool {
	// If there's no target nodes, then there's definitely not enough resources available
	if len(targetNodes) == 0 {
		return false
	}
	totalAvailableTargetCpu := int64(0)
	totalAvailableTargetMemory := int64(0)
	// Get resources available in target nodes
	for _, targetNode := range targetNodes {
		availableTargetCpu := targetNode.Status.Allocatable.Cpu().MilliValue()
		availableTargetMemory := targetNode.Status.Allocatable.Memory().MilliValue()
		podsInNode, err := GetPodsInNode(targetNode.Name)
		if err != nil {
			continue
		}
		for _, podInNode := range podsInNode {
			for _, container := range podInNode.Spec.Containers {
				if container.Resources.Requests.Cpu() != nil {
					// Subtract the cpu request of the pod from the node's total allocatable
					availableTargetCpu -= container.Resources.Requests.Cpu().MilliValue()
				}
				if container.Resources.Requests.Memory() != nil {
					// Subtract the cpu request of the pod from the node's total allocatable
					totalAvailableTargetMemory -= container.Resources.Requests.Memory().MilliValue()
				}
			}
		}

		totalAvailableTargetCpu += availableTargetCpu
		totalAvailableTargetMemory += availableTargetMemory
	}
	cpuNeeded := int64(0)
	memoryNeeded := int64(0)
	// Get resources requested in old node
	podsInNode, err := GetPodsInNode(oldNode.Name)
	if err != nil {
		log.Printf("Unable to determine resources needed for old node, assuming that enough resources are available")
		return true
	}
	for _, podInNode := range podsInNode {
		for _, container := range podInNode.Spec.Containers {
			if container.Resources.Requests.Cpu() != nil {
				// Subtract the cpu request of the pod from the node's total allocatable
				cpuNeeded += container.Resources.Requests.Cpu().MilliValue()
			}
			if container.Resources.Requests.Memory() != nil {
				// Subtract the cpu request of the pod from the node's total allocatable
				memoryNeeded += container.Resources.Requests.Memory().MilliValue()
			}
		}
	}
	leftOverCpu := totalAvailableTargetCpu - cpuNeeded
	leftOverMemory := totalAvailableTargetMemory - memoryNeeded
	return leftOverCpu > 0 && leftOverMemory > 0
}
