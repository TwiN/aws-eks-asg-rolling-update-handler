package cloud

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

func GetServices() (ec2iface.EC2API, autoscalingiface.AutoScalingAPI, error) {
	awsSession, err := session.NewSession(&aws.Config{Region: aws.String("us-west-2")})
	if err != nil {
		return nil, nil, err
	}
	return ec2.New(awsSession), autoscaling.New(awsSession), nil
}

func DescribeAutoScalingGroupsByNames(svc autoscalingiface.AutoScalingAPI, names []string) ([]*autoscaling.Group, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice(names),
	}
	result, err := svc.DescribeAutoScalingGroups(input)
	if err != nil {
		return nil, err
	}
	return result.AutoScalingGroups, nil
}

func DescribeLaunchTemplateByID(svc ec2iface.EC2API, id string) (*ec2.LaunchTemplate, error) {
	input := &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateIds: []*string{
			aws.String(id),
		},
	}
	return DescribeLaunchTemplate(svc, input)
}

func DescribeLaunchTemplateByName(svc ec2iface.EC2API, name string) (*ec2.LaunchTemplate, error) {
	input := &ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{
			aws.String(name),
		},
	}
	return DescribeLaunchTemplate(svc, input)
}

func DescribeLaunchTemplate(svc ec2iface.EC2API, input *ec2.DescribeLaunchTemplatesInput) (*ec2.LaunchTemplate, error) {
	templatesOutput, err := svc.DescribeLaunchTemplates(input)
	descriptiveMsg := fmt.Sprintf("%v / %v", input.LaunchTemplateIds, input.LaunchTemplateNames)
	if err != nil {
		return nil, fmt.Errorf("unable to get description for Launch Template %s: %v", descriptiveMsg, err)
	}
	if len(templatesOutput.LaunchTemplates) < 1 {
		return nil, nil
	}
	return templatesOutput.LaunchTemplates[0], nil
}
