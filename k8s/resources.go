package k8s

import (
	"fmt"
	drain "github.com/openshift/kubernetes-drain"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

const (
	ScaleDownDisabledAnnotationKey                = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	RollingUpdateStartedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/started-at"
	RollingUpdateDrainedTimestampAnnotationKey    = "aws-eks-asg-rolling-update-handler/drained-at"
	RollingUpdateTerminatedTimestampAnnotationKey = "aws-eks-asg-rolling-update-handler/terminated-at"
	HostNameAnnotationKey                         = "kubernetes.io/hostname"
)

func GetNodes() ([]v1.Node, error) {
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

func GetPodsInNode(node string) ([]v1.Pod, error) {
	client, err := CreateClient()
	podList, err := client.CoreV1().Pods("").List(metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func GetNodeByHostName(hostName string) (*v1.Node, error) {
	client, err := CreateClient()
	if err != nil {
		return nil, err
	}
	api := client.CoreV1().Nodes()
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

func UpdateNode(node *v1.Node) error {
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
	node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	// set options and drain nodes
	return drain.Drain(client, []*v1.Node{node}, &drain.DrainOptions{
		IgnoreDaemonsets:   ignoreDaemonSets,
		GracePeriodSeconds: -1,
		Force:              true,
		DeleteLocalData:    deleteLocalData,
		Timeout:            3 * time.Minute,
	})
}
