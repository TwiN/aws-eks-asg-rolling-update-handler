package cloud

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

var (
	ErrCannotIncreaseDesiredCountAboveMax = errors.New("cannot increase ASG desired size above max ASG size")
)

// GetServices returns an instance of a EC2 client with a session as well as
// an instance of an Autoscaling client with a session
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
		MaxRecords:            aws.Int64(100),
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

// DescribeEnabledAutoScalingGroupsByTags Gets AutoScalingGroups that match the given tags
func DescribeEnabledAutoScalingGroupsByTags(svc autoscalingiface.AutoScalingAPI, autodiscoveryTags string) ([]*autoscaling.Group, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	var result []*autoscaling.Group
	err := svc.DescribeAutoScalingGroupsPages(input, func(page *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) bool {
		tagFilter := func(tagDescriptions []*autoscaling.TagDescription) bool {
			var matches []bool
			for _, tag := range strings.Split(autodiscoveryTags, ",") {
				kv := strings.Split(tag, "=")
				match := false
				for _, tagDescription := range tagDescriptions {
					if aws.StringValue(tagDescription.Key) == kv[0] && aws.StringValue(tagDescription.Value) == kv[1] {
						match = true
						break
					}
				}
				matches = append(matches, match)
			}
			for _, match := range matches {
				if !match {
					return false
				}
			}
			return true
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

// IncrementAutoScalingGroupDesiredCount retrieves the latest definition of the ASG and increments its current
// desired capacity by 1. The reason why we retrieve the ASG again even though we already have it is to avoid a
// scenario in which the ASG had already been scaled up or down since the last time it was retrieved.
// See https://github.com/TwiN/aws-eks-asg-rolling-update-handler/issues/129 for more information.
func IncrementAutoScalingGroupDesiredCount(svc autoscalingiface.AutoScalingAPI, autoScalingGroupName string) error {
	latestASGs, err := DescribeAutoScalingGroupsByNames(svc, []string{autoScalingGroupName})
	if err != nil {
		return fmt.Errorf("failed to retrieve latest asg with name '%s': %w", autoScalingGroupName, err)
	}
	if len(latestASGs) != 1 {
		// ASG names are unique per region and account, so if there isn't exactly one ASG, there's a problem.
		return errors.New("failed to retrieve latest asg with name: " + autoScalingGroupName)
	}
	asg := latestASGs[0]
	newDesiredCapacity := aws.Int64Value(asg.DesiredCapacity) + 1
	if newDesiredCapacity > aws.Int64Value(asg.MaxSize) {
		return ErrCannotIncreaseDesiredCountAboveMax
	}
	desiredInput := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: asg.AutoScalingGroupName,
		DesiredCapacity:      aws.Int64(newDesiredCapacity),
		HonorCooldown:        aws.Bool(true),
	}
	_, err = svc.SetDesiredCapacity(desiredInput)
	if err != nil {
		return fmt.Errorf("unable to increase ASG %s desired count to %d: %w", autoScalingGroupName, newDesiredCapacity, err)
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
