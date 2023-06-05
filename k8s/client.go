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
	AnnotationRollingUpdateCordonedTimestamp   = "aws-eks-asg-rolling-update-handler.twin.sh/cordoned-at"
	AnnotationRollingUpdateDrainedTimestamp    = "aws-eks-asg-rolling-update-handler.twin.sh/drained-at"
	AnnotationRollingUpdateTerminatedTimestamp = "aws-eks-asg-rolling-update-handler.twin.sh/terminated-at"

	nodesCacheKey = "nodes"
)

var (
	cache = gocache.NewCache().WithMaxSize(1000).WithEvictionPolicy(gocache.LeastRecentlyUsed)
)

type ClientAPI interface {
	GetNodes() ([]v1.Node, error)
	GetPodsInNode(nodeName string) ([]v1.Pod, error)
	GetNodeByAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error)
	FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error)
	UpdateNode(node *v1.Node) error
	Cordon(nodeName string) error
	Drain(nodeName string, ignoreDaemonSets, deleteEmptyDirData bool, podTerminationGracePeriod int) error
}

type Client struct {
	client kubernetes.Interface
}

// NewClient creates a new Client
func NewClient(client kubernetes.Interface) *Client {
	return &Client{
		client: client,
	}
}

// GetNodes retrieves all nodes from the cluster
func (k *Client) GetNodes() ([]v1.Node, error) {
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
func (k *Client) GetPodsInNode(node string) ([]v1.Pod, error) {
	podList, err := k.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector:   "spec.nodeName=" + node,
		ResourceVersion: "0",
	})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}

// GetNodeByAutoScalingInstance gets the Kubernetes node matching an AWS AutoScaling instance
// Because we cannot filter by spec.providerID, the entire list of nodes is fetched every time
// this function is called
func (k *Client) GetNodeByAutoScalingInstance(instance *autoscaling.Instance) (*v1.Node, error) {
	nodes, err := k.GetNodes()
	if err != nil {
		return nil, err
	}
	return k.FilterNodeByAutoScalingInstance(nodes, instance)
}

// FilterNodeByAutoScalingInstance extracts the Kubernetes node belonging to a given AWS instance from a list of nodes
func (k *Client) FilterNodeByAutoScalingInstance(nodes []v1.Node, instance *autoscaling.Instance) (*v1.Node, error) {
	providerId := fmt.Sprintf("aws:///%s/%s", aws.StringValue(instance.AvailabilityZone), aws.StringValue(instance.InstanceId))
	for _, node := range nodes {
		if node.Spec.ProviderID == providerId {
			return &node, nil
		}
	}
	return nil, fmt.Errorf("node with providerID \"%s\" not found", providerId)
}

// UpdateNode updates a node
func (k *Client) UpdateNode(node *v1.Node) error {
	api := k.client.CoreV1().Nodes()
	_, err := api.Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

// Cordon disables scheduling new pods onto the given node
func (k *Client) Cordon(nodeName string) error {
	node, err := k.client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	drainer := &drain.Helper{
		Client: k.client,
		Ctx:    context.TODO(),
	}
	if err := drain.RunCordonOrUncordon(drainer, node, true); err != nil {
		log.Printf("[%s][CORDONER] Failed to cordon node: %v", node.Name, err)
		return err
	}
	return nil
}

// Drain gracefully deletes all pods from a given node
func (k *Client) Drain(nodeName string, ignoreDaemonSets, deleteEmptyDirData bool, podTerminationGracePeriod int) error {
	node, err := k.client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	drainer := &drain.Helper{
		Client:              k.client,
		Force:               true, // Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet
		IgnoreAllDaemonSets: ignoreDaemonSets,
		DeleteEmptyDirData:  deleteEmptyDirData,
		GracePeriodSeconds:  podTerminationGracePeriod,
		Timeout:             5 * time.Minute,
		Ctx:                 context.TODO(),
		Out:                 drainLogger{NodeName: nodeName},
		ErrOut:              drainLogger{NodeName: nodeName},
		OnPodDeletedOrEvicted: func(pod *v1.Pod, usingEviction bool) {
			log.Printf("[%s][DRAINER] evicted pod %s/%s", nodeName, pod.Namespace, pod.Name)
		},
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
