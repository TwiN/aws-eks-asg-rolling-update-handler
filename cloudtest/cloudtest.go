package cloudtest

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type MockEC2Service struct {
	ec2iface.EC2API

	Counter   map[string]int64
	Templates []*ec2.LaunchTemplate
}

func NewMockEC2Service(templates []*ec2.LaunchTemplate) *MockEC2Service {
	return &MockEC2Service{
		Counter:   make(map[string]int64),
		Templates: templates,
	}
}

func (m *MockEC2Service) DescribeLaunchTemplates(_ *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error) {
	m.Counter["DescribeLaunchTemplates"]++
	output := &ec2.DescribeLaunchTemplatesOutput{
		LaunchTemplates: m.Templates,
	}
	return output, nil
}

func (m *MockEC2Service) DescribeLaunchTemplateByID(input *ec2.DescribeLaunchTemplatesInput) (*ec2.LaunchTemplate, error) {
	m.Counter["DescribeLaunchTemplateByID"]++
	for _, template := range m.Templates {
		if template.LaunchTemplateId == input.LaunchTemplateIds[0] {
			return template, nil
		}
		if template.LaunchTemplateName == input.LaunchTemplateNames[0] {
			return template, nil
		}
	}
	return nil, errors.New("not found")
}

func CreateTestEc2Instance(id string) *ec2.Instance {
	instance := &ec2.Instance{
		InstanceId: aws.String(id),
	}
	return instance
}

type MockAutoScalingService struct {
	autoscalingiface.AutoScalingAPI

	Counter           map[string]int64
	AutoScalingGroups map[string]*autoscaling.Group
}

func NewMockAutoScalingService(autoScalingGroups []*autoscaling.Group) *MockAutoScalingService {
	service := &MockAutoScalingService{
		Counter:           make(map[string]int64),
		AutoScalingGroups: make(map[string]*autoscaling.Group),
	}
	for _, autoScalingGroup := range autoScalingGroups {
		service.AutoScalingGroups[aws.StringValue(autoScalingGroup.AutoScalingGroupName)] = autoScalingGroup
	}
	return service
}

func (m *MockAutoScalingService) TerminateInstanceInAutoScalingGroup(_ *autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	m.Counter["TerminateInstanceInAutoScalingGroup"]++
	return &autoscaling.TerminateInstanceInAutoScalingGroupOutput{}, nil
}

func (m *MockAutoScalingService) DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	m.Counter["DescribeAutoScalingGroups"]++
	var autoScalingGroups []*autoscaling.Group
	for _, autoScalingGroupName := range input.AutoScalingGroupNames {
		for _, autoScalingGroup := range m.AutoScalingGroups {
			if aws.StringValue(autoScalingGroupName) == aws.StringValue(autoScalingGroup.AutoScalingGroupName) {
				autoScalingGroups = append(autoScalingGroups, autoScalingGroup)
			}
		}
	}
	return &autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: autoScalingGroups,
	}, nil
}

func (m *MockAutoScalingService) SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	m.Counter["SetDesiredCapacity"]++
	m.AutoScalingGroups[aws.StringValue(input.AutoScalingGroupName)].SetDesiredCapacity(aws.Int64Value(input.DesiredCapacity))
	return &autoscaling.SetDesiredCapacityOutput{}, nil
}

func (m *MockAutoScalingService) UpdateAutoScalingGroup(_ *autoscaling.UpdateAutoScalingGroupInput) (*autoscaling.UpdateAutoScalingGroupOutput, error) {
	m.Counter["UpdateAutoScalingGroup"]++
	return &autoscaling.UpdateAutoScalingGroupOutput{}, nil
}

func CreateTestAutoScalingGroup(name, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, instances []*autoscaling.Instance) *autoscaling.Group {
	asg := &autoscaling.Group{
		AutoScalingGroupName: aws.String(name),
		Instances:            instances,
		DesiredCapacity:      aws.Int64(int64(len(instances))),
		MinSize:              aws.Int64(0),
		MaxSize:              aws.Int64(999),
	}
	if len(launchConfigurationName) != 0 {
		asg.SetLaunchConfigurationName(launchConfigurationName)
	}
	if launchTemplateSpecification != nil {
		asg.SetLaunchTemplate(launchTemplateSpecification)
	}
	return asg
}

func CreateTestAutoScalingInstance(id, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, lifeCycleState string) *autoscaling.Instance {
	instance := &autoscaling.Instance{
		LifecycleState: aws.String(lifeCycleState),
		InstanceId:     aws.String(id),
	}
	if len(launchConfigurationName) != 0 {
		instance.SetLaunchConfigurationName(launchConfigurationName)
	}
	if launchTemplateSpecification != nil {
		instance.SetLaunchTemplate(launchTemplateSpecification)
	}
	return instance
}
