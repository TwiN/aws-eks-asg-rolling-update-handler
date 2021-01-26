package k8s

import (
	"log"

	"github.com/aws/aws-sdk-go/service/autoscaling"
	"k8s.io/api/core/v1"
)

// CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes calculates the resources available in the target nodes
// and compares them with the resources that would be required if the old node were to be drained
//
// This is not fool proof: 2 targetNodes with 1G available in each would cause the assumption that you can fit
// a 2G pod in the targetNodes when you obviously can't (you'd need 1 node with 2G available, not 2 with 1G)
// That's alright, because the purpose is to provide a smooth rolling upgrade, not a flawless experience,  and
// while the latter is definitely possible, it would slow down the process by quite a bit. In a way, this is
// the beauty of co-existing with the cluster autoscaler; an extra node will be spun up to handle the leftovers,
// if any.
func CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(kubernetesClient KubernetesClientApi, oldNode *v1.Node, targetNodes []*v1.Node) bool {
	totalAvailableTargetCpu := int64(0)
	totalAvailableTargetMemory := int64(0)
	// Get resources available in target nodes
	for _, targetNode := range targetNodes {
		availableTargetCpu := targetNode.Status.Allocatable.Cpu().MilliValue()
		availableTargetMemory := targetNode.Status.Allocatable.Memory().MilliValue()
		podsInNode, err := kubernetesClient.GetPodsInNode(targetNode.Name)
		if err != nil {
			continue
		}
		for _, podInNode := range podsInNode {
			// Skip pods that have terminated (e.g. "Evicted" pods that haven't been cleaned up)
			if podInNode.Status.Phase == v1.PodFailed {
				continue
			}
			for _, container := range podInNode.Spec.Containers {
				if container.Resources.Requests.Cpu() != nil {
					// Subtract the cpu request of the pod from the node's total allocatable cpu
					availableTargetCpu -= container.Resources.Requests.Cpu().MilliValue()
				}
				if container.Resources.Requests.Memory() != nil {
					// Subtract the memory request of the pod from the node's total allocatable memory
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
	podsInNode, err := kubernetesClient.GetPodsInNode(oldNode.Name)
	if err != nil {
		log.Printf("Unable to determine resources needed for old node, assuming that enough resources are available")
		return true
	}
	for _, podInNode := range podsInNode {
		// Skip pods that have terminated (e.g. "Evicted" pods that haven't been cleaned up)
		if podInNode.Status.Phase == v1.PodFailed {
			continue
		}
		// Ignore DaemonSets in the old node, because these pods will also be present in the target nodes
		hasDaemonSetOwnerReference := false
		for _, owner := range podInNode.GetOwnerReferences() {
			if owner.Kind == "DaemonSet" {
				hasDaemonSetOwnerReference = true
				break
			}
		}
		if hasDaemonSetOwnerReference {
			continue
		}
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
	return leftOverCpu >= 0 && leftOverMemory >= 0
}

// AnnotateNodeByAwsAutoScalingInstance adds an annotation to the Kubernetes node represented by a given AWS instance
func AnnotateNodeByAwsAutoScalingInstance(kubernetesClient KubernetesClientApi, instance *autoscaling.Instance, key, value string) error {
	node, err := kubernetesClient.GetNodeByAwsAutoScalingInstance(instance)
	if err != nil {
		return err
	}
	annotations := node.GetAnnotations()
	if currentValue := annotations[key]; currentValue != value {
		annotations[key] = value
		node.SetAnnotations(annotations)
		err = kubernetesClient.UpdateNode(node)
		if err != nil {
			return err
		}
	}
	return nil
}
