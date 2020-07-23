package k8s

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/openshift/cluster-api/pkg/drain"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

type KubernetesClientApi interface {
	GetNodes() ([]v1.Node, error)
	GetPodsInNode(node string) ([]v1.Pod, error)
	GetNodeByAwsAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error)
	FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error)
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

// GetNodeByAwsAutoScalingInstance gets the Kubernetes node matching an AWS AutoScaling instance
// Because we cannot filter by spec.providerID, the entire list of nodes is fetched every time
// this function is called
func (k *KubernetesClient) GetNodeByAwsAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error) {
	////For some reason, we can't filter by spec.providerID
	//api := k.client.CoreV1().Nodes()
	//nodeList, err := api.List(metav1.ListOptions{
	//	//LabelSelector: fmt.Sprintf("%s=%s", HostNameAnnotationKey, aws.StringValue(instance.InstanceId)),
	//	FieldSelector: fmt.Sprintf("spec.providerID=aws:///%s/%s", aws.StringValue(instance.AvailabilityZone), aws.StringValue(instance.InstanceId)),
	//	Limit:         1,
	//})
	//if err != nil {
	//	return nil, err
	//}
	//if len(nodeList.Items) == 0 {
	//	return nil, fmt.Errorf("nodes with AWS instance id \"%s\" not found", aws.StringValue(instance.InstanceId))
	//}
	//return &nodeList.Items[0], nil
	nodes, err := k.GetNodes()
	if err != nil {
		return nil, err
	}
	return k.FilterNodeByAutoScalingInstance(nodes, instance)
}

func (k *KubernetesClient) FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error) {
	providerId := fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.AvailabilityZone), aws.StringValue(instance.InstanceId))
	for _, node := range nodes {
		if node.Spec.ProviderID == providerId {
			return &node, nil
		}
	}
	return nil, fmt.Errorf("node with providerID \"%s\" not found", providerId)
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
	return drain.Drain(k.client, []*v1.Node{node}, &drain.DrainOptions{
		IgnoreDaemonsets:   ignoreDaemonSets,
		GracePeriodSeconds: -1,
		Force:              true,
		Logger:             &drainLogger{NodeName: nodeName},
		DeleteLocalData:    deleteLocalData,
		Timeout:            5 * time.Minute,
	})
}

type drainLogger struct {
	NodeName string
}

func (l *drainLogger) Log(v ...interface{}) {
	log.Println(fmt.Sprintf("[%s][DRAINER]", l.NodeName), fmt.Sprint(v...))
}

func (l *drainLogger) Logf(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("[%s][DRAINER]", l.NodeName), fmt.Sprintf(format, v...))
}
