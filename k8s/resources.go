package k8s

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ScaleDownDisabledAnnotationKey      = "cluster-autoscaler.kubernetes.io/scale-down-disabled"
	RollingUpdateStartTimeAnnotationKey = "aws-eks-asg-rolling-update-handler/start-time"
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
