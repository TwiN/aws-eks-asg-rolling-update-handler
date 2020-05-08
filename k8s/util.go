package k8s

import (
	"k8s.io/api/core/v1"
	"log"
)

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
	podsInNode, err := kubernetesClient.GetPodsInNode(oldNode.Name)
	if err != nil {
		log.Printf("Unable to determine resources needed for old node, assuming that enough resources are available")
		return true
	}
	for _, podInNode := range podsInNode {
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

func AnnotateNodeByHostName(kubernetesClient KubernetesClientApi, hostName, key, value string) error {
	node, err := kubernetesClient.GetNodeByHostName(hostName)
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
