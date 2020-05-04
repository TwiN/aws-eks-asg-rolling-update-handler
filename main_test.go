package main

import (
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/cloudtest"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"testing"
)

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsOutdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsUpdated(t *testing.T) {
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "v1", nil, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v1"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "Healthy")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "Healthy")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "Healthy")
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
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", outdatedLaunchTemplate, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, &cloudtest.MockEC2Service{Templates: []*ec2.LaunchTemplate{updatedEc2LaunchTemplate}})
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
	instance := cloudtest.CreateTestAutoScalingInstance("instance", "", updatedLaunchTemplate, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, &cloudtest.MockEC2Service{Templates: []*ec2.LaunchTemplate{updatedEc2LaunchTemplate}})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstances_withLaunchConfigurationWhenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := cloudtest.CreateTestAutoScalingInstance("old-1", "v1", nil, "Healthy")
	secondInstance := cloudtest.CreateTestAutoScalingInstance("old-2", "v1", nil, "Healthy")
	thirdInstance := cloudtest.CreateTestAutoScalingInstance("new", "v2", nil, "Healthy")

	asg := cloudtest.CreateTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance})

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
