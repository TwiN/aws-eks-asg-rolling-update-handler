package cloudtest

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

type MockEC2Service struct {
	ec2iface.EC2API
	Templates []*ec2.LaunchTemplate
}

func (m *MockEC2Service) DescribeLaunchTemplates(_ *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error) {
	output := &ec2.DescribeLaunchTemplatesOutput{
		LaunchTemplates: m.Templates,
	}
	return output, nil
}

func (m *MockEC2Service) DescribeLaunchTemplateByID(input *ec2.DescribeLaunchTemplatesInput) (*ec2.LaunchTemplate, error) {
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

func CreateTestAutoScalingGroup(name, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, instances []*autoscaling.Instance) *autoscaling.Group {
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

func CreateTestAutoScalingInstance(id, launchConfigurationName string, launchTemplateSpecification *autoscaling.LaunchTemplateSpecification, healthStatus string) *autoscaling.Instance {
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

func CreateTestEc2Instance(id string) *ec2.Instance {
	instance := &ec2.Instance{
		InstanceId: aws.String(id),
	}
	return instance
}
