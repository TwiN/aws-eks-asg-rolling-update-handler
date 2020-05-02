package main

import (
	"errors"
	"fmt"
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/cloud"
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/config"
	"github.com/TwinProduction/aws-eks-asg-rolling-update-handler/k8s"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"log"
)

func main() {
	err := config.Initialize()
	if err != nil {
		log.Fatalf("Unable to initialize configuration: %s", err.Error())
	}
	//for {
	if err := run(); err != nil {
		log.Printf("Error during execution: %s", err.Error())
	}
	//    time.Sleep(time.Minute)
	//}
}

func run() error {
	cfg := config.Get()
	_, err := k8s.CreateClient()
	if err != nil {
		return fmt.Errorf("unable to create Kubernetes client: %s", err.Error())
	}

	ec2Service, autoScalingService, err := cloud.GetServices()
	if err != nil {
		return fmt.Errorf("unable to create AWS services: %s", err.Error())
	}

	autoScalingGroups, err := cloud.DescribeAutoScalingGroupsByNames(autoScalingService, cfg.AutoScalingGroupNames)
	if err != nil {
		return fmt.Errorf("unable to describe AutoScalingGroups: %s", err.Error())
	}

	HandleRollingUpgrade(ec2Service, autoScalingService, autoScalingGroups)
	//nodes, err := k8s.GetNodes()
	//if err != nil {
	//	return fmt.Errorf("unable to get nodes: %s", err.Error())
	//}
	//
	//for _, node := range nodes {
	//	log.Printf("%v", node)
	//}
	return nil
}

func HandleRollingUpgrade(ec2Service ec2iface.EC2API, autoScalingService autoscalingiface.AutoScalingAPI, autoScalingGroups []*autoscaling.Group) {
	for _, autoScalingGroup := range autoScalingGroups {
		outdatedInstances, updatedInstances, err := SeparateOutdatedFromUpdatedInstances(autoScalingGroup, ec2Service)
		if err != nil {
			log.Printf("[%s] Unable to separate outdated instances from updated instances: %v", *autoScalingGroup.AutoScalingGroupName, err.Error())
			log.Printf("[%s] Skipping", *autoScalingGroup.AutoScalingGroupName)
			continue
		}

		log.Printf("outdatedInstances: %v", outdatedInstances)
		log.Printf("updatedInstances: %v", updatedInstances)

		if len(outdatedInstances) == 0 {
			log.Printf("[%s] All instances up to date", *autoScalingGroup.AutoScalingGroupName)
			continue
		}

		// Check if outdated nodes in k8s have been marked with annotation from aws-eks-asg-rolling-update-handler

		// Check if ASG hit max, and then decide what to do (patience or violence)
	}
}

func SeparateOutdatedFromUpdatedInstances(asg *autoscaling.Group, ec2Svc ec2iface.EC2API) ([]*autoscaling.Instance, []*autoscaling.Instance, error) {
	targetLaunchConfiguration := asg.LaunchConfigurationName
	targetLaunchTemplate := asg.LaunchTemplate
	if targetLaunchTemplate == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		log.Printf("[%s] using mixed instances policy launch template", *asg.AutoScalingGroupName)
		targetLaunchTemplate = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
	}
	if targetLaunchTemplate != nil {
		return SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(targetLaunchTemplate, asg.Instances, ec2Svc)
	} else if targetLaunchConfiguration != nil {
		return SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(targetLaunchConfiguration, asg.Instances)
	}
	return nil, nil, errors.New("AutoScalingGroup has neither launch template nor launch configuration")
}

func SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(targetLaunchTemplate *autoscaling.LaunchTemplateSpecification, instances []*autoscaling.Instance, ec2Svc ec2iface.EC2API) ([]*autoscaling.Instance, []*autoscaling.Instance, error) {
	// we are using LaunchTemplate. Unlike LaunchConfiguration, you can have two nodes in the ASG
	//  with the same LT name, same ID but different versions, so need to check version.
	//  they even can have the same version, if the version is `$Latest` or `$Default`, so need
	//  to get actual versions for each
	var (
		oldInstances   []*autoscaling.Instance
		newInstances   []*autoscaling.Instance
		targetTemplate *ec2.LaunchTemplate
		err            error
	)
	switch {
	case targetLaunchTemplate.LaunchTemplateId != nil && *targetLaunchTemplate.LaunchTemplateId != "":
		if targetTemplate, err = cloud.DescribeLaunchTemplateByID(ec2Svc, *targetLaunchTemplate.LaunchTemplateId); err != nil {
			return nil, nil, fmt.Errorf("error retrieving information about launch template ID %s: %v", *targetLaunchTemplate.LaunchTemplateId, err)
		}
	case targetLaunchTemplate.LaunchTemplateName != nil && *targetLaunchTemplate.LaunchTemplateName != "":
		if targetTemplate, err = cloud.DescribeLaunchTemplateByName(ec2Svc, *targetLaunchTemplate.LaunchTemplateName); err != nil {
			return nil, nil, fmt.Errorf("error retrieving information about launch template name %s: %v", *targetLaunchTemplate.LaunchTemplateName, err)
		}
	default:
		return nil, nil, fmt.Errorf("invalid launch template name")
	}
	// extra safety check
	if targetTemplate == nil {
		return nil, nil, fmt.Errorf("no template found")
	}
	// now we can loop through each node and compare
	for _, i := range instances {
		switch {
		case i.LaunchTemplate == nil:
			// has no launch template at all
			oldInstances = append(oldInstances, i)
		case aws.StringValue(i.LaunchTemplate.LaunchTemplateName) != aws.StringValue(targetLaunchTemplate.LaunchTemplateName):
			oldInstances = append(oldInstances, i)
		case aws.StringValue(i.LaunchTemplate.LaunchTemplateId) != aws.StringValue(targetLaunchTemplate.LaunchTemplateId):
			oldInstances = append(oldInstances, i)
		// name and id match, just need to check versions
		case !compareLaunchTemplateVersions(targetTemplate, targetLaunchTemplate, i.LaunchTemplate):
			oldInstances = append(oldInstances, i)
		default:
			newInstances = append(newInstances, i)
		}
	}

	return oldInstances, newInstances, nil
}

func SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(targetLaunchConfigurationName *string, instances []*autoscaling.Instance) ([]*autoscaling.Instance, []*autoscaling.Instance, error) {
	var (
		oldInstances []*autoscaling.Instance
		newInstances []*autoscaling.Instance
	)
	for _, i := range instances {
		if i.LaunchConfigurationName != nil && *i.LaunchConfigurationName == *targetLaunchConfigurationName {
			newInstances = append(newInstances, i)
		} else {
			oldInstances = append(oldInstances, i)
		}
	}
	return oldInstances, newInstances, nil
}

// compareLaunchTemplateVersions compare two launch template versions and see if they match
// can handle `$Latest` and `$Default` by resolving to the actual version in use
func compareLaunchTemplateVersions(targetTemplate *ec2.LaunchTemplate, lt1, lt2 *autoscaling.LaunchTemplateSpecification) bool {
	// if both versions do not start with `$`, then just compare
	if lt1 == nil && lt2 == nil {
		return true
	}
	if (lt1 == nil && lt2 != nil) || (lt1 != nil && lt2 == nil) {
		return false
	}
	if lt1.Version == nil && lt2.Version == nil {
		return true
	}
	if (lt1.Version == nil && lt2.Version != nil) || (lt1.Version != nil && lt2.Version == nil) {
		return false
	}
	// if either version starts with `$`, then resolve to actual version from LaunchTemplate
	var lt1version, lt2version string
	switch *lt1.Version {
	case "$Default":
		lt1version = fmt.Sprintf("%d", *targetTemplate.DefaultVersionNumber)
	case "$Latest":
		lt1version = fmt.Sprintf("%d", *targetTemplate.LatestVersionNumber)
	default:
		lt1version = *lt1.Version
	}
	switch *lt2.Version {
	case "$Default":
		lt2version = fmt.Sprintf("%d", *targetTemplate.DefaultVersionNumber)
	case "$Latest":
		lt2version = fmt.Sprintf("%d", *targetTemplate.LatestVersionNumber)
	default:
		lt2version = *lt2.Version
	}
	return lt1version == lt2version
}
