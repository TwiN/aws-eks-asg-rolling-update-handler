package k8s

import (
	"k8s.io/api/core/v1"
	"testing"
)

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(t *testing.T) {
	// allocatable cpu & memory aren't used for the old node.
	// They're only used by the target nodes (newNode, in this case) to calculate if the leftover resources from moving
	// the pods from the old node to the new node are positive (if the leftover is negative, it means there's not enough
	// space in the target nodes)
	oldNode := createTestNode("old-node", "0m", "0m")
	newNode := createTestNode("new-node-1", "1000m", "1000Mi")
	oldNodePod := createTestPod("old-pod-1", oldNode.Name, "100m", "100Mi")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodePod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{&newNode})
	if !hasEnoughResources {
		t.Error("should've had enough space in node")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes_whenNotEnoughSpaceInNewNodes(t *testing.T) {
	oldNode := createTestNode("old-node", "0m", "0m")
	newNode := createTestNode("new-node-1", "1000m", "1000Mi")
	oldNodePod := createTestPod("old-pod-1", oldNode.Name, "200m", "200Mi")
	newNodePod := createTestPod("new-pod-1", newNode.Name, "900m", "200Mi")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodePod, newNodePod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{&newNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes_withMultiplePods(t *testing.T) {
	oldNode := createTestNode("old-node", "0m", "0m")
	newNode := createTestNode("new-node-1", "1000m", "1000Mi")
	oldNodeFirstPod := createTestPod("old-pod-1", oldNode.Name, "300m", "0")
	oldNodeSecondPod := createTestPod("old-pod-2", oldNode.Name, "300m", "0")
	oldNodeThirdPod := createTestPod("old-pod-3", oldNode.Name, "300m", "0")
	newNodePod := createTestPod("new-pod-1", newNode.Name, "200m", "200Mi")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, newNodePod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{&newNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes_withMultipleTargetNodes(t *testing.T) {
	oldNode := createTestNode("old-node", "0m", "0m")
	firstNewNode := createTestNode("new-node-1", "1000m", "1000Mi")
	secondNewNode := createTestNode("new-node-2", "1000m", "1000Mi")
	oldNodeFirstPod := createTestPod("old-node-pod-1", oldNode.Name, "500m", "0")
	oldNodeSecondPod := createTestPod("old-node-pod-2", oldNode.Name, "500m", "0")
	oldNodeThirdPod := createTestPod("old-node-pod-3", oldNode.Name, "500m", "0")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode, firstNewNode, secondNewNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{&firstNewNode, &secondNewNode})
	if !hasEnoughResources {
		t.Error("should've had enough space in node")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 3 {
		t.Error("GetPodInNode should've been called thrice")
	}
}

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes_withPodsSpreadAcrossMultipleTargetNodes(t *testing.T) {
	oldNode := createTestNode("old-node", "0m", "0m")
	firstNewNode := createTestNode("new-node-1", "1000m", "1000Mi")
	secondNewNode := createTestNode("new-node-2", "1000m", "1000Mi")
	firstNewNodePod := createTestPod("new-node-1-pod-1", oldNode.Name, "0", "300Mi")
	secondNewNodePod := createTestPod("new-node-2-pod-1", oldNode.Name, "0", "300Mi")
	oldNodeFirstPod := createTestPod("old-node-pod-1", oldNode.Name, "0", "500Mi")
	oldNodeSecondPod := createTestPod("old-node-pod-2", oldNode.Name, "0", "500Mi")
	oldNodeThirdPod := createTestPod("old-node-pod-3", oldNode.Name, "0", "500Mi")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode, firstNewNode, secondNewNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, firstNewNodePod, secondNewNodePod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{&firstNewNode, &secondNewNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 3 {
		t.Error("GetPodInNode should've been called thrice")
	}
}

func TestCheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes_withNoTargetNodes(t *testing.T) {
	oldNode := createTestNode("old-node", "0m", "0m")
	oldNodePod := createTestPod("old-node-pod-1", oldNode.Name, "500Mi", "500Mi")
	mockKubernetesClient := NewMockKubernetesClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})

	hasEnoughResources := CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(mockKubernetesClient, &oldNode, []*v1.Node{})
	if hasEnoughResources {
		t.Error("there's no target nodes; there definitely shouldn't have been enough space")
	}
	if mockKubernetesClient.counter["GetPodsInNode"] != 0 {
		t.Error("GetPodInNode shouldn't have been called")
	}
}
