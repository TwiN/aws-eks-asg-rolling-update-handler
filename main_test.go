package main

import (
	"testing"

	"github.com/TwiN/aws-eks-asg-rolling-update-handler/cloudtest"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/config"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/k8s"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/k8stest"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	v1 "k8s.io/api/core/v1"
)

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsOutdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsUpdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v1"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 2 {
		t.Error("2 instances should've been outdated")
	}
	if len(updated) != 1 {
		t.Error("1 instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceIsOutdated(t *testing.T) {
	outdatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v2"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     updatedLaunchTemplate.LaunchTemplateId,
		LaunchTemplateName:   updatedLaunchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", outdatedLaunchTemplate, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate("test", updatedLaunchTemplate, nil, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceIsOutdatedDueToMixedInstancesPolicyInstanceTypeGettingRemoved(t *testing.T) {
	launchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     launchTemplate.LaunchTemplateId,
		LaunchTemplateName:   launchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", launchTemplate, "InService")
	instance.SetInstanceType("c5n.2xlarge")
	overrides := []*autoscaling.LaunchTemplateOverrides{
		{InstanceType: aws.String("c5.2xlarge")},
		{InstanceType: aws.String("c5d.2xlarge")},
	}
	// Notice: The instance's instance type isn't part of the overrides.
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate("test", launchTemplate, overrides, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceIsUpdated(t *testing.T) {
	updatedLaunchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     updatedLaunchTemplate.LaunchTemplateId,
		LaunchTemplateName:   updatedLaunchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", updatedLaunchTemplate, "InService")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate("test", updatedLaunchTemplate, nil, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceWithMixedInstancesPolicyIsUpdated(t *testing.T) {
	launchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     launchTemplate.LaunchTemplateId,
		LaunchTemplateName:   launchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", launchTemplate, "InService")
	instance.SetInstanceType("c5d.2xlarge")
	overrides := []*autoscaling.LaunchTemplateOverrides{
		{InstanceType: aws.String("c5.2xlarge")},
		{InstanceType: aws.String("c5d.2xlarge")},
	}
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate("test", launchTemplate, overrides, []*autoscaling.Instance{instance}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate_whenInstanceWithMixedInstancesPolicyAndOverrideIsUpdated(t *testing.T) {
	launchTemplate := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("id"),
		LaunchTemplateName: aws.String("name"),
		Version:            aws.String("v1"),
	}
	updatedEc2LaunchTemplate := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(10),
		LaunchTemplateId:     launchTemplate.LaunchTemplateId,
		LaunchTemplateName:   launchTemplate.LaunchTemplateName,
	}
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", launchTemplate, "InService")
	instance.SetInstanceType("c5d.2xlarge")
	instanceWithLaunchTemplateOverride := cloudtest.CreateTestAutoScalingInstance("instance", "", launchTemplate, "InService")
	instanceWithLaunchTemplateOverride.SetInstanceType("c5d.2xlarge")
	overrides := []*autoscaling.LaunchTemplateOverrides{
		{InstanceType: aws.String("c5.2xlarge"), LaunchTemplateSpecification: launchTemplate},
		{InstanceType: aws.String("c5d.2xlarge")},
	}
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate("test", launchTemplate, overrides, []*autoscaling.Instance{instance, instanceWithLaunchTemplateOverride}, cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{updatedEc2LaunchTemplate}))
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 2 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstances_withLaunchConfigurationWhenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "InService")

	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance}, false)

	outdated, updated, err := SeparateOutdatedFromUpdatedInstances(asg, nil)
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 2 {
		t.Error("2 instances should've been outdated")
	}
	if len(updated) != 1 {
		t.Error("1 instance should've been outdated")
	}
}

func TestHandleRollingUpgrade(t *testing.T) {
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, false)

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false, v1.PodRunning)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	err := HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateStartedTimestamp]; !ok {
		t.Error("Node should've been annotated with", k8s.AnnotationRollingUpdateStartedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateTerminatedTimestamp)
	}

	// Second run (ASG's desired capacity gets increased)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Third run (Nothing changed)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByAutoScalingInstance should return not found))
	newInstance.SetLifecycleState("InService")
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode("new-node-1", aws.StringValue(newInstance.AvailabilityZone), aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockClient.Nodes[newNode.Name] = newNode
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Seventh run (Kubelet is ready to accept new pods. Old node gets drained and terminated)
	newNode = mockClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockClient.Nodes[newNode.Name] = newNode
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; !ok {
		t.Error("Node should've been terminated")
	}
}

func TestHandleRollingUpgrade_withLaunchTemplate(t *testing.T) {
	oldLaunchTemplateSpecification := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("lt1"),
		LaunchTemplateName: aws.String("lt1"),
		Version:            aws.String("1"),
	}
	newLaunchTemplateSpecification := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("lt1"),
		LaunchTemplateName: aws.String("lt1"),
		Version:            aws.String("2"),
	}
	lt := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(1),
		LaunchTemplateId:     aws.String("lt1"),
		LaunchTemplateName:   aws.String("lt1"),
	}
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "", oldLaunchTemplateSpecification, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "", newLaunchTemplateSpecification, []*autoscaling.Instance{oldInstance}, false)

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false, v1.PodRunning)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})
	mockEc2Service := cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{lt})
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateStartedTimestamp]; !ok {
		t.Error("Node should've been annotated with", k8s.AnnotationRollingUpdateStartedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateTerminatedTimestamp)
	}

	// Second run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Third run (Nothing changed)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "", newLaunchTemplateSpecification, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByAutoScalingInstance should return not found))
	newInstance.SetLifecycleState("InService")
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode("new-node-1", aws.StringValue(newInstance.AvailabilityZone), aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Seventh run (Kubelet is ready to accept new pods. Old node gets drained and terminated)
	newNode = mockClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; !ok {
		t.Error("Node should've been terminated")
	}
}

func TestHandleRollingUpgrade_withLaunchTemplateWhenLaunchTemplateDidNotUpdate(t *testing.T) {
	launchTemplateSpecification := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("lt1"),
		LaunchTemplateName: aws.String("lt1"),
		Version:            aws.String("1"),
	}
	lt := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(1),
		LaunchTemplateId:     aws.String("lt1"),
		LaunchTemplateName:   aws.String("lt1"),
	}
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "", launchTemplateSpecification, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "", launchTemplateSpecification, []*autoscaling.Instance{oldInstance}, false)

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{})
	mockEc2Service := cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{lt})
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (No changes, no updates)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockClient.Counter["UpdateNode"] != 0 {
		t.Error("The LT hasn't been updated, therefore nothing should've changed")
	}
}

func TestHandleRollingUpgrade_withEnoughPodsToRequireTwoNewNodes(t *testing.T) {
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, false)

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodeFirstPod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "300m", "300Mi", false, v1.PodRunning)
	oldNodeSecondPod := k8stest.CreateTestPod("old-pod-2", oldNode.Name, "300m", "300Mi", false, v1.PodRunning)
	oldNodeThirdPod := k8stest.CreateTestPod("old-pod-3", oldNode.Name, "300m", "300Mi", false, v1.PodRunning)
	oldNodeFourthPod := k8stest.CreateTestPod("old-pod-4", oldNode.Name, "300m", "300Mi", false, v1.PodRunning)
	// This pod should be ignored, because the pod.Status.Phase is v1.PodFailed
	oldNodeFifthPod := k8stest.CreateTestPod("old-pod-5-evicted", oldNode.Name, "99999m", "99999Mi", false, v1.PodFailed)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodeFirstPod, oldNodeSecondPod, oldNodeThirdPod, oldNodeFourthPod, oldNodeFifthPod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateStartedTimestamp]; !ok {
		t.Error("Node should've been annotated with", k8s.AnnotationRollingUpdateStartedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; ok {
		t.Error("Node shouldn't have been terminated yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateTerminatedTimestamp)
	}

	// Second run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Third run (Nothing changed)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByAutoScalingInstance should return not found))
	newInstance.SetLifecycleState("InService")
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode("new-node-1", aws.StringValue(newInstance.AvailabilityZone), aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Seventh run (Kubelet is ready to accept new pods)
	newNode = mockClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockClient.Nodes[newNode.Name] = newNode
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Eight run (ASG's desired capacity gets increased)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 2 {
		t.Error("ASG should've been increased again")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 3 {
		t.Error("The desired capacity of the ASG should've been increased to 3")
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; ok {
		t.Error("Node shouldn't have been drained yet, therefore shouldn't have been annotated with", k8s.AnnotationRollingUpdateDrainedTimestamp)
	}

	// Ninth run (fast-forward new instance, node and kubelet ready to accept. Old node gets drained and terminated)
	newSecondInstance := cloudtest.CreateTestAutoScalingInstance("new-2", "v2", nil, "InService")
	asg.Instances = append(asg.Instances, newSecondInstance)
	newSecondNode := k8stest.CreateTestNode("new-node-2", aws.StringValue(newSecondInstance.AvailabilityZone), aws.StringValue(newSecondInstance.InstanceId), "1000m", "1000Mi")
	newSecondNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockClient.Nodes[newSecondNode.Name] = newSecondNode
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateDrainedTimestamp]; !ok {
		t.Error("Node should've been drained")
	}
	if _, ok := oldNode.GetAnnotations()[k8s.AnnotationRollingUpdateTerminatedTimestamp]; !ok {
		t.Error("Node should've been terminated")
	}
}

// The mixed instance policy is not part of the launch template; it's part of the ASG itself.
// This means that not only must we check the launch template version (it doesn't change in this test), but
// we must also check if the instance's instance type is part of the MixedInstancesPolicy's instance types.
// If it isn't, then it means the ASG has been modified, and the instance is old.
func TestHandleRollingUpgrade_withMixedInstancePolicyWhenOneOfTheInstanceTypesOverrideChanges(t *testing.T) {
	launchTemplateSpecification := &autoscaling.LaunchTemplateSpecification{
		LaunchTemplateId:   aws.String("lt1"),
		LaunchTemplateName: aws.String("lt1"),
		Version:            aws.String("1"),
	}
	lt := &ec2.LaunchTemplate{
		DefaultVersionNumber: aws.Int64(1),
		LatestVersionNumber:  aws.Int64(1),
		LaunchTemplateId:     aws.String("lt1"),
		LaunchTemplateName:   aws.String("lt1"),
	}
	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "", launchTemplateSpecification, "InService")
	// The LT has NOT changed, but we're setting withMixedInstancesPolicy to true
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "", launchTemplateSpecification, []*autoscaling.Instance{oldInstance}, true)
	// We set the instance type to something isn't the default instance type, because the first one has the same value as the
	// Launch template version, meaning that modifying that one would likely trigger a new version to be created.
	// What we're trying to test here is whether we're able to trigger a rolling update on an instance type that is no
	// longer part of the MixedInstancesPolicy overrides
	oldInstance.SetInstanceType(aws.StringValue(asg.MixedInstancesPolicy.LaunchTemplate.Overrides[1].InstanceType))

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{})
	mockEc2Service := cloudtest.NewMockEC2Service([]*ec2.LaunchTemplate{lt})
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Nothing changed)
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockClient.Counter["UpdateNode"] != 0 {
		t.Error("Nothing should've changed")
	}

	// Suddenly, the ASG's MixedInstancePolicy gets updated, and only the first instance type override is kept
	// The second instance type is the one that our old instance uses
	asg.MixedInstancesPolicy.SetLaunchTemplate(&autoscaling.LaunchTemplate{
		LaunchTemplateSpecification: asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification,
		Overrides:                   asg.MixedInstancesPolicy.LaunchTemplate.Overrides[0:1],
	})

	// Second run
	HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if mockClient.Counter["UpdateNode"] != 1 {
		t.Error("The old instance's instance type is no longer part of the ASG's MixedInstancePolicy's LaunchTemplate overrides, therefore, it is outdated and should've been annotated")
	}
}

func TestHasAcceptableNumberOfUpdatedNonReadyNodes(t *testing.T) {
	// false: there's too many non-ready nodes
	// true:  there's an acceptable amount of non-ready nodes given how many ready nodes there are
	if HasAcceptableNumberOfUpdatedNonReadyNodes(100, 0) {
		t.Error("100NR/0R ready should not be acceptable")
	}
	if HasAcceptableNumberOfUpdatedNonReadyNodes(50, 50) {
		t.Error("50NR/50R should not be acceptable")
	}
	if HasAcceptableNumberOfUpdatedNonReadyNodes(6, 10000) {
		t.Error("6NR/10000R should not be acceptable, because MaximumNumberOfUpdatedNonReadyNodes is set to", MaximumNumberOfUpdatedNonReadyNodes)
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(5, 10000) {
		t.Error("5NR/10000R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(4, 100) {
		t.Error("4NR/100R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(1, 99) {
		t.Error("1NR/99R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(0, 100) {
		t.Error("0NR/100R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(0, 1) {
		t.Error("0NR/1R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(0, 0) {
		t.Error("0NR/0R should be acceptable")
	}
	if !HasAcceptableNumberOfUpdatedNonReadyNodes(1, 11) {
		t.Error("1NR/11R should be acceptable")
	}
}

func TestHandleRollingUpgrade_withEagerCordoning(t *testing.T) {
	config.Set(nil, true, true, true, false)
	defer config.Set(nil, true, true, false, false)

	oldInstance1 := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	oldInstance2 := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	oldInstance3 := cloudtest.CreateTestAutoScalingInstance("old-3", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance1, oldInstance2, oldInstance3}, false)

	oldNode1 := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance1.AvailabilityZone), aws.StringValue(oldInstance1.InstanceId), "1000m", "1000Mi")
	oldNode2 := k8stest.CreateTestNode("old-node-2", aws.StringValue(oldInstance2.AvailabilityZone), aws.StringValue(oldInstance2.InstanceId), "1000m", "1000Mi")
	oldNode3 := k8stest.CreateTestNode("old-node-3", aws.StringValue(oldInstance3.AvailabilityZone), aws.StringValue(oldInstance3.InstanceId), "1000m", "1000Mi")
	oldNodePod1 := k8stest.CreateTestPod("old-pod-1", oldNode1.Name, "600m", "600Mi", false, v1.PodRunning)
	oldNodePod2 := k8stest.CreateTestPod("old-pod-2", oldNode2.Name, "600m", "600Mi", false, v1.PodRunning)
	oldNodePod3 := k8stest.CreateTestPod("old-pod-3", oldNode3.Name, "600m", "600Mi", false, v1.PodRunning)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode1, oldNode2, oldNode3}, []v1.Pod{oldNodePod1, oldNodePod2, oldNodePod3})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	err := HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockClient.Counter["UpdateNode"] != 3 {
		t.Error("Node should've been annotated as started, meaning that UpdateNode should've been called once")
	}
	// Make sure that all nodes were "eagerly cordoned"
	if mockClient.Counter["Cordon"] != 3 {
		t.Error("Node should've been annotated, meaning that Cordon should've been called thrice, but was called", mockClient.Counter["Cordon"], "times")
	}
}

func TestHandleRollingUpgrade_withEagerCordoningDisabled(t *testing.T) {
	// explicitly setting this, but eager cordoning is disabled by default anyways
	config.Set(nil, true, true, false, true)
	defer config.Set(nil, true, true, true, false)

	oldInstance1 := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	oldInstance2 := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance1, oldInstance2}, false)

	oldNode1 := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance1.AvailabilityZone), aws.StringValue(oldInstance1.InstanceId), "1000m", "1000Mi")
	oldNode2 := k8stest.CreateTestNode("old-node-2", aws.StringValue(oldInstance2.AvailabilityZone), aws.StringValue(oldInstance2.InstanceId), "1000m", "1000Mi")
	oldNodePod1 := k8stest.CreateTestPod("old-pod-1", oldNode1.Name, "600m", "600Mi", false, v1.PodRunning)
	oldNodePod2 := k8stest.CreateTestPod("old-pod-2", oldNode2.Name, "600m", "600Mi", false, v1.PodRunning)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode1, oldNode2}, []v1.Pod{oldNodePod1, oldNodePod2})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	err := HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockClient.Counter["UpdateNode"] != 2 {
		t.Error("Nodes should've been annotated as started, meaning that UpdateNode should've been called twice")
	}
	// Make sure that all nodes were NOT "eagerly cordoned"
	if mockClient.Counter["Cordon"] != 0 {
		t.Error("Eager cordoning is not enabled, so no node should have been cordoned on the first execution")
	}
}

func TestHandleRollingUpgrade_withExcludeFromExternalLoadBalancers(t *testing.T) {
	config.Set(nil, true, true, false, true)
	defer config.Set(nil, true, true, false, false)

	oldInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "InService")
	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{oldInstance}, false)

	oldNode := k8stest.CreateTestNode("old-node-1", aws.StringValue(oldInstance.AvailabilityZone), aws.StringValue(oldInstance.InstanceId), "1000m", "1000Mi")
	oldNodePod := k8stest.CreateTestPod("old-pod-1", oldNode.Name, "100m", "100Mi", false, v1.PodRunning)

	mockClient := k8stest.NewMockClient([]v1.Node{oldNode}, []v1.Pod{oldNodePod})
	mockEc2Service := cloudtest.NewMockEC2Service(nil)
	mockAutoScalingService := cloudtest.NewMockAutoScalingService([]*autoscaling.Group{asg})

	// First run (Node rollout process gets marked as started)
	err := HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockClient.Counter["UpdateNode"] != 1 {
		t.Error("Node should've been annotated, meaning that UpdateNode should've been called once")
	}

	// Second run (ASG's desired capacity gets increased)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("ASG should've been increased because there's no updated nodes yet")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've been increased to 2")
	}

	// Third run (Nothing changed)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}
	asg = mockAutoScalingService.AutoScalingGroups[aws.StringValue(asg.AutoScalingGroupName)]
	if aws.Int64Value(asg.DesiredCapacity) != 2 {
		t.Error("The desired capacity of the ASG should've stayed at 2")
	}

	// Fourth run (new instance has been registered to ASG, but is pending)
	newInstance := cloudtest.CreateTestAutoScalingInstance("new-1", "v2", nil, "Pending")
	asg.Instances = append(asg.Instances, newInstance)
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	if mockAutoScalingService.Counter["SetDesiredCapacity"] != 1 {
		t.Error("Desired capacity shouldn't have been updated")
	}

	// Fifth run (new instance is now InService, but node has still not joined cluster (GetNodeByAutoScalingInstance should return not found))
	newInstance.SetLifecycleState("InService")
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}

	// Sixth run (new instance has joined the cluster, but Kubelet isn't ready to accept pods yet)
	newNode := k8stest.CreateTestNode("new-node-1", aws.StringValue(newInstance.AvailabilityZone), aws.StringValue(newInstance.InstanceId), "1000m", "1000Mi")
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}
	mockClient.Nodes[newNode.Name] = newNode
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}

	// Seventh run (Kubelet is ready to accept new pods. Old node gets drained and terminated)
	newNode = mockClient.Nodes[newNode.Name]
	newNode.Status.Conditions = []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionTrue}}
	mockClient.Nodes[newNode.Name] = newNode
	err = HandleRollingUpgrade(mockClient, mockEc2Service, mockAutoScalingService, []*autoscaling.Group{asg})
	if err != nil {
		t.Error("unexpected error:", err)
	}
	oldNode = mockClient.Nodes[oldNode.Name]
	if _, ok := oldNode.GetLabels()[k8s.LabelExcludeFromExternalLoadBalancers]; !ok {
		t.Error("Node should've been labeled")
	}
}
