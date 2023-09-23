package k8s

import (
	"testing"

	"github.com/TwiN/aws-eks-asg-rolling-update-handler/k8stest"
	"k8s.io/api/core/v1"
)

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(t *testing.T) {
	// allocatable cpu & memory aren't used for the old node.
	// They're only used by the target nodes (newNode, in this case) to calculate if the leftover resources from moving
	// the pods from the old node to the new node are positive (if the leftover is negative, it means there's not enough
	// space in the target nodes)
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2a", "i-034fa1dfbfd35f8bb", "0m", "0m")
	newNode := k8stest.CreateTestNode("new-node-1", "us-west-2b", "i-07550830aef9e4179", "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{&newNode})
	if !hasEnoughResources {
		t.Error("should've had enough space in node")
	}
	if mockClient.Counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_whenNotEnoughSpaceInNewNodes(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2a", "i-034fa1dfbfd35f8bb", "0m", "0m")
	newNode := k8stest.CreateTestNode("new-node-1", "us-west-2c", "i-0b22d79604221412c", "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "200m", "200Mi", false, v1.PodRunning)
	newNodePod := k8stest.CreateTestPod("new-pod-1", newNode.Name, "900m", "200Mi", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodePod, newNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{&newNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockClient.Counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_withMultiplePods(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2c", "i-0b22d79604221412c", "0m", "0m")
	newNode := k8stest.CreateTestNode("new-node-1", "us-west-2b", "i-07550830aef9e4179", "1000m", "1000Mi")
	oldNodeFirstPod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "300m", "0", false, v1.PodRunning)
	oldNodeSecondPod := k8stest.CreateTestPod("old-pod-2", oldNode.Name, "300m", "0", false, v1.PodRunning)
	oldNodeThirdPod := k8stest.CreateTestPod("old-pod-3", oldNode.Name, "300m", "0", false, v1.PodRunning)
	newNodePod := k8stest.CreateTestPod("new-pod-1", newNode.Name, "200m", "200Mi", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode, newNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, newNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{&newNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockClient.Counter["GetPodsInNode"] != 2 {
		t.Error("GetPodInNode should've been called twice")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_withMultipleTargetNodes(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2b", "i-07550830aef9e4179", "0m", "0m")
	firstNewNode := k8stest.CreateTestNode("new-node-1", "us-west-2a", "i-034fa1dfbfd35f8bb", "1000m", "1000Mi")
	secondNewNode := k8stest.CreateTestNode("new-node-2", "us-west-2b", "i-0918aff89347cef0c", "1000m", "1000Mi")
	oldNodeFirstPod := k8stest.CreateTestPod("old-node-pod-1", oldNode.Name, "500m", "0", false, v1.PodRunning)
	oldNodeSecondPod := k8stest.CreateTestPod("old-node-pod-2", oldNode.Name, "500m", "0", false, v1.PodRunning)
	oldNodeThirdPod := k8stest.CreateTestPod("old-node-pod-3", oldNode.Name, "500m", "0", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode, firstNewNode, secondNewNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{&firstNewNode, &secondNewNode})
	if !hasEnoughResources {
		t.Error("should've had enough space in node")
	}
	if mockClient.Counter["GetPodsInNode"] != 3 {
		t.Error("GetPodInNode should've been called thrice")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_withPodsSpreadAcrossMultipleTargetNodes(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2a", "i-034fa1dfbfd35f8bb", "0m", "0m")
	firstNewNode := k8stest.CreateTestNode("new-node-1", "us-west-2a", "i-07550830aef9e4179", "1000m", "1000Mi")
	secondNewNode := k8stest.CreateTestNode("new-node-2", "us-west-2a", "i-0147ad0816c210dae", "1000m", "1000Mi")
	firstNewNodePod := k8stest.CreateTestPod("new-node-1-pod-1", oldNode.Name, "0", "300Mi", false, v1.PodRunning)
	secondNewNodePod := k8stest.CreateTestPod("new-node-2-pod-1", oldNode.Name, "0", "300Mi", false, v1.PodRunning)
	oldNodeFirstPod := k8stest.CreateTestPod("old-node-pod-1", oldNode.Name, "0", "500Mi", false, v1.PodRunning)
	oldNodeSecondPod := k8stest.CreateTestPod("old-node-pod-2", oldNode.Name, "0", "500Mi", false, v1.PodRunning)
	oldNodeThirdPod := k8stest.CreateTestPod("old-node-pod-3", oldNode.Name, "0", "500Mi", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode, firstNewNode, secondNewNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, firstNewNodePod, secondNewNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{&firstNewNode, &secondNewNode})
	if hasEnoughResources {
		t.Error("shouldn't have had enough space in node")
	}
	if mockClient.Counter["GetPodsInNode"] != 3 {
		t.Error("GetPodInNode should've been called thrice")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_withNoTargetNodes(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2a", "i-034fa1dfbfd35f8bb", "0m", "0m")
	oldNodePod := k8stest.CreateTestPod("old-node-pod-1", oldNode.Name, "500Mi", "500Mi", false, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{})
	if hasEnoughResources {
		t.Error("there's no target nodes; there definitely shouldn't have been enough space")
	}
}

func TestCheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode_withNoTargetNodesButOldNodeOnlyHasPodsFromDaemonSets(t *testing.T) {
	oldNode := k8stest.CreateTestNode("old-node", "us-west-2a", "i-034fa1dfbfd35f8bb", "0m", "0m")
	oldNodePod := k8stest.CreateTestPod("old-node-pod-1", oldNode.Name, "500Mi", "500Mi", true, v1.PodRunning)
	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})

	hasEnoughResources := CheckIfUpdatedNodesHaveEnoughResourcesToScheduleAllPodsFromOldNode(mockClient, &oldNode, []*v1.Node{})
	if !hasEnoughResources {
		t.Error("there's no target nodes, but the only pods in the old node are from daemon sets")
	}
}
