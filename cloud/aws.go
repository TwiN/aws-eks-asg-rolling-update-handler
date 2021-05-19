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
	"strings"
)

var (
	ErrCannotIncreaseDesiredCountAboveMax = errors.New("cannot increase ASG desired size above max ASG size")
)

func GetServices(awsRegion string) (ec2iface.EC2API, autoscalingiface.AutoScalingAPI, error) {
	awsSession, err := session.NewSession(&aws.Config{Region: aws.String(awsRegion)})
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

func filterAutoScalingGroupsByTag(autoScalingGroups []*autoscaling.Group, filter func([]*autoscaling.TagDescription) bool) (ret []*autoscaling.Group) {
	for _, autoScalingGroup := range autoScalingGroups {
		if filter(autoScalingGroup.Tags) {
			ret = append(ret, autoScalingGroup)
		}
	}
	return
}

// DescribeEnabledAutoScalingGroupsByClusterName Gets cluster AutoScalingGroups that are enabled
// See: https://docs.aws.amazon.com/eks/latest/userguide/cluster-autoscaler.html
func DescribeEnabledAutoScalingGroupsByClusterName(svc autoscalingiface.AutoScalingAPI, clusterName string) ([]*autoscaling.Group, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	var result []*autoscaling.Group
	err := svc.DescribeAutoScalingGroupsPages(input, func(page *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
		tagFilter := func(tagDescriptions []*autoscaling.TagDescription) bool {
			clusterNameTag := false
			enabledTag := false
			for _, tagDescription := range tagDescriptions {
				clusterNameTag = clusterNameTag || (aws.StringValue(tagDescription.Key) == fmt.Sprintf("k8s.io/cluster-autoscaler/%s", clusterName) && aws.StringValue(tagDescription.Value) == "owned")
				enabledTag = enabledTag || (aws.StringValue(tagDescription.Key) == "k8s.io/cluster-autoscaler/enabled" && strings.ToLower(aws.StringValue(tagDescription.Value)) == "true")
			}
			return clusterNameTag && enabledTag

		}
		result = append(result, filterAutoScalingGroupsByTag(page.AutoScalingGroups, tagFilter)...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
	if count > aws.Int64Value(asg.MaxSize) {
		return ErrCannotIncreaseDesiredCountAboveMax
	}
	desiredInput := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: asg.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(count),
		HonorCooldown:        aws.Bool(true),
	}
	_, err := svc.SetDesiredCapacity(desiredInput)
	if err != nil {
		return fmt.Errorf("unable to increase ASG %s desired count to %d: %v", aws.StringValue(asg.AutoScalingGroupName), count, err)
	}
	return nil
}

func TerminateEc2Instance(svc autoscalingiface.AutoScalingAPI, instance *autoscaling.Instance, shouldDecrementDesiredCapacity bool) error {
	_, err := svc.TerminateInstanceInAutoScalingGroup(&autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     instance.InstanceId,
		ShouldDecrementDesiredCapacity: aws.Bool(shouldDecrementDesiredCapacity),
	})
	return err
}
