package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/TwiN/gocache/v2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

const (
	AnnotationRollingUpdateStartedTimestamp    = "aws-eks-asg-rolling-update-handler.twin.sh/started-at"
	AnnotationRollingUpdateDrainedTimestamp    = "aws-eks-asg-rolling-update-handler.twin.sh/drained-at"
	AnnotationRollingUpdateTerminatedTimestamp = "aws-eks-asg-rolling-update-handler.twin.sh/terminated-at"

	nodesCacheKey = "nodes"
)

var (
	cache = gocache.NewCache().WithMaxSize(1000).WithEvictionPolicy(gocache.LeastRecentlyUsed)
)

type KubernetesClientApi interface {
	GetNodes() ([]v1.Node, error)
	GetPodsInNode(node string) ([]v1.Pod, error)
	GetNodeByAwsAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error)
	FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error)
	UpdateNode(node *v1.Node) error
	Drain(nodeName string, ignoreDaemonSets, deleteEmptyDirData bool) error
}

type KubernetesClient struct {
	client *kubernetes.Clientset
}

// NewKubernetesClient creates a new KubernetesClient
func NewKubernetesClient(client *kubernetes.Clientset) *KubernetesClient {
	return &KubernetesClient{
		client: client,
	}
}

// GetNodes retrieves all nodes from the cluster
func (k *KubernetesClient) GetNodes() ([]v1.Node, error) {
	nodes, exists := cache.Get(nodesCacheKey)
	if exists {
		if v1Nodes, ok := nodes.([]v1.Node); ok {
			// Return cached nodes
			return v1Nodes, nil
		} else {
			log.Println("[k8s.GetNodes] Failed to cast cached nodes to []v1.Node; retrieving nodes from API instead")
			cache.Delete(nodesCacheKey)
		}
	}
	nodeList, err := k.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	cache.SetWithTTL(nodesCacheKey, nodeList.Items, 10*time.Second)
	return nodeList.Items, nil
}

// GetPodsInNode retrieves all pods from a given node
func (k *KubernetesClient) GetPodsInNode(node string) ([]v1.Pod, error) {
	podList, err := k.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
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

// FilterNodeByAutoScalingInstance extracts the Kubernetes node belonging to a given AWS instance from a list of nodes
func (k *KubernetesClient) FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error) {
	providerId := fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.AvailabilityZone), aws.StringValue(instance.InstanceId))
	for _, node := range nodes {
		if node.Spec.ProviderID == providerId {
			return &node, nil
		}
	}
	return nil, fmt.Errorf("node with providerID \"%s\" not found", providerId)
}

// UpdateNode updates a node
func (k *KubernetesClient) UpdateNode(node *v1.Node) error {
	api := k.client.CoreV1().Nodes()
	_, err := api.Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

// Drain gracefully deletes all pods from a given node
func (k *KubernetesClient) Drain(nodeName string, ignoreDaemonSets, deleteEmptyDirData bool) error {
	node, err := k.client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	drainer := &drain.Helper{
		Client:              k.client,
		Force:               true,
		IgnoreAllDaemonSets: ignoreDaemonSets,
		DeleteEmptyDirData:  deleteEmptyDirData,
		GracePeriodSeconds:  -1,
		Timeout:             5 * time.Minute,
		Out:                 drainLogger{NodeName: nodeName},
		ErrOut:              drainLogger{NodeName: nodeName},
		OnPodDeletedOrEvicted: func(pod *v1.Pod, usingEviction bool) {
			log.Printf("[%s][DRAINER] evicted pod %s/%s", nodeName, pod.Namespace, pod.Name)
		},
	}
	if err := drain.RunCordonOrUncordon(drainer, node, true); err != nil {
		log.Printf("[%s][DRAINER] Failed to cordon node: %v", node.Name, err)
		return err
	}
	if err := drain.RunNodeDrain(drainer, node.Name); err != nil {
		log.Printf("[%s][DRAINER] Failed to drain node: %v", node.Name, err)
		return err
	}
	return nil
}

type drainLogger struct {
	NodeName string
}

func (l drainLogger) Write(p []byte) (n int, err error) {
	log.Printf("[%s][DRAINER] %s", l.NodeName, string(p))
	return len(p), nil
}
