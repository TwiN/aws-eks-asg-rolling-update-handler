package cloud

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

var (
	ErrCannotIncreaseDesiredCountAboveMax = errors.New("cannot incease ASG desired size above max ASG size")
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
	descriptiveMsg := fmt.Sprintf("%v / %v", aws.StringValueSlice(input.LaunchTemplateIds), aws.StringValueSlice(input.LaunchTemplateNames))
	if err != nil {
		return nil, fmt.Errorf("unable to get description for Launch Templates %s: %v", descriptiveMsg, err)
	}
	if len(templatesOutput.LaunchTemplates) < 1 {
		return nil, nil
	}
	return templatesOutput.LaunchTemplates[0], nil
}

func SetAutoScalingGroupDesiredCount(svc autoscalingiface.AutoScalingAPI, asg *autoscaling.Group, count int64) error {
	if count > *asg.MaxSize {
		return ErrCannotIncreaseDesiredCountAboveMax
	}
	desiredInput := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: asg.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(count),
		HonorCooldown:        aws.Bool(true),
	}
	_, err := svc.SetDesiredCapacity(desiredInput)
	if err != nil {
		return fmt.Errorf("unable to increase ASG %s desired count to %d: %v", *asg.AutoScalingGroupName, count, err)
	}
	return nil
}

func TerminateEc2Instance(svc autoscalingiface.AutoScalingAPI, instance *autoscaling.Instance) error {
	_, err := svc.TerminateInstanceInAutoScalingGroup(&autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     aws.String(*instance.InstanceId),
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	})
	return err
}
