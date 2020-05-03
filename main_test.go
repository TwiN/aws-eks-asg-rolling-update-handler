package main

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"testing"
)

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsOutdated(t *testing.T) {
	instance := createTestAutoScalingInstance("instance", "v1", nil, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v2"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 1 || len(updated) != 0 {
		t.Error("Instance should've been outdated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenInstanceIsUpdated(t *testing.T) {
	instance := createTestAutoScalingInstance("instance", "v1", nil, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(aws.String("v1"), []*autoscaling.Instance{instance})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration_whenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := createTestAutoScalingInstance("old-1", "v1", nil, "Healthy")
	secondInstance := createTestAutoScalingInstance("old-2", "v1", nil, "Healthy")
	thirdInstance := createTestAutoScalingInstance("new", "v2", nil, "Healthy")
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
	instance := createTestAutoScalingInstance("instance", "", outdatedLaunchTemplate, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, &mockEc2Svc{templates: []*ec2.LaunchTemplate{updatedEc2LaunchTemplate}})
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
	instance := createTestAutoScalingInstance("instance", "", updatedLaunchTemplate, "Healthy")
	outdated, updated, err := SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(updatedLaunchTemplate, []*autoscaling.Instance{instance}, &mockEc2Svc{templates: []*ec2.LaunchTemplate{updatedEc2LaunchTemplate}})
	if err != nil {
		t.Fatal("Shouldn't have returned an error, but returned:", err)
	}
	if len(outdated) != 0 || len(updated) != 1 {
		t.Error("Instance should've been updated")
	}
}

func TestSeparateOutdatedFromUpdatedInstances_withLaunchConfigurationWhenOneInstanceIsUpdatedAndTwoInstancesAreOutdated(t *testing.T) {
	firstInstance := createTestAutoScalingInstance("old-1", "v1", nil, "Healthy")
	secondInstance := createTestAutoScalingInstance("old-2", "v1", nil, "Healthy")
	thirdInstance := createTestAutoScalingInstance("new", "v2", nil, "Healthy")

	asg := createTestAutoScalingGroup("asg", "v2", nil, []*autoscaling.Instance{firstInstance, secondInstance, thirdInstance})

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

func createTestAutoScalingGroup(name string, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, instances []*autoscaling.Instance) *autoscaling.Group {
	asg := &autoscaling.Group{
		AutoScalingGroupName: aws.String(name),
		Instances:            instances,
	}
	if len(launchConfigurationName) != 0 {
		asg.SetLaunchConfigurationName(launchConfigurationName)
	}
	if launchTemplateSpecification != nil {
		asg.SetLaunchTemplate(launchTemplateSpecification)
	}
	return asg
}

func createTestAutoScalingInstance(id, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, healthStatus string) *autoscaling.Instance {
	instance := &autoscaling.Instance{
		HealthStatus: aws.String(healthStatus),
		InstanceId:   aws.String(id),
	}
	if len(launchConfigurationName) != 0 {
		instance.SetLaunchConfigurationName(launchConfigurationName)
	}
	if launchTemplateSpecification != nil {
		instance.SetLaunchTemplate(launchTemplateSpecification)
	}
	return instance
}

func createTestEc2Instance(id string) *ec2.Instance {
	instance := &ec2.Instance{
		InstanceId: aws.String(id),
	}
	return instance
}

type mockEc2Svc struct {
	ec2iface.EC2API
	templates []*ec2.LaunchTemplate
}

func (m *mockEc2Svc) DescribeLaunchTemplates(in *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error) {
	fmt.Println(in)
	fmt.Println(m.templates)
	output := &ec2.DescribeLaunchTemplatesOutput{
		LaunchTemplates: m.templates,
	}
	return output, nil
}

func (m *mockEc2Svc) DescribeLaunchTemplateByID(in *ec2.DescribeLaunchTemplatesInput) (*ec2.LaunchTemplate, error) {
	for _, template := range m.templates {
		if template.LaunchTemplateId == in.LaunchTemplateIds[0] {
			return template, nil
		}
		if template.LaunchTemplateName == in.LaunchTemplateNames[0] {
			return template, nil
		}
	}
	return nil, errors.New("not found")
}
