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
	"k8s.io/api/core/v1"
	"log"
	"time"
)

func main() {
	err := config.Initialize()
	if err != nil {
		log.Fatalf("Unable to initialize configuration: %s", err.Error())
	}
	for {
		if err := run(); err != nil {
			log.Printf("Error during execution: %s", err.Error())
		}
		log.Println("Sleeping for 10 seconds")
		time.Sleep(10 * time.Second)
	}
}

func run() error {
	cfg := config.Get()
	client, err := k8s.CreateClientSet()
	if err != nil {
		return fmt.Errorf("unable to create Kubernetes client: %s", err.Error())
	}
	kubernetesClient := k8s.NewKubernetesClient(client)

	ec2Service, autoScalingService, err := cloud.GetServices()
	if err != nil {
		return fmt.Errorf("unable to create AWS services: %s", err.Error())
	}

	autoScalingGroups, err := cloud.DescribeAutoScalingGroupsByNames(autoScalingService, cfg.AutoScalingGroupNames)
	if err != nil {
		return fmt.Errorf("unable to describe AutoScalingGroups: %s", err.Error())
	}

	HandleRollingUpgrade(kubernetesClient, ec2Service, autoScalingService, autoScalingGroups)
	return nil
}

func HandleRollingUpgrade(kubernetesClient k8s.KubernetesClientApi, ec2Service ec2iface.EC2API, autoScalingService autoscalingiface.AutoScalingAPI, autoScalingGroups []*autoscaling.Group) {
	for _, autoScalingGroup := range autoScalingGroups {
		outdatedInstances, updatedInstances, err := SeparateOutdatedFromUpdatedInstances(autoScalingGroup, ec2Service)
		if err != nil {
			log.Printf("[%s] Unable to separate outdated instances from updated instances: %v", *autoScalingGroup.AutoScalingGroupName, err.Error())
			log.Printf("[%s] Skipping", *autoScalingGroup.AutoScalingGroupName)
			continue
		}

		//log.Printf("outdatedInstances: %v", outdatedInstances)
		//log.Printf("updatedInstances: %v", updatedInstances)

		// Get the updated and ready nodes from the list of updated instances
		// This will be used to determine if the desired number of updated instances need to scale up or not
		// We also use this to clean up, if necessary
		var updatedReadyNodes []*v1.Node
		numberOfNonReadyNodesOrInstances := 0
		for _, updatedInstance := range updatedInstances {
			if *updatedInstance.LifecycleState != "InService" {
				numberOfNonReadyNodesOrInstances++
				log.Printf("[%s][%s] Skipping because instance is not in LifecycleState 'InService', but is in '%s' instead", *autoScalingGroup.AutoScalingGroupName, *updatedInstance.InstanceId, *updatedInstance.LifecycleState)
				continue
			}
			updatedNode, err := kubernetesClient.GetNodeByHostName(*updatedInstance.InstanceId)
			if err != nil {
				log.Printf("[%s][%s] Unable to get updated node from Kubernetes: %v", *autoScalingGroup.AutoScalingGroupName, *updatedInstance.InstanceId, err.Error())
				log.Printf("[%s][%s] Skipping", *autoScalingGroup.AutoScalingGroupName, *updatedInstance.InstanceId)
				continue
			}
			// Check if Kubelet is ready to accept pods on that node
			conditions := updatedNode.Status.Conditions
			if kubeletCondition := conditions[len(conditions)-1]; kubeletCondition.Type == v1.NodeReady && kubeletCondition.Status == v1.ConditionTrue {
				updatedReadyNodes = append(updatedReadyNodes, updatedNode)
			} else {
				numberOfNonReadyNodesOrInstances++
			}
			// Cleaning up
			// This is an edge case, but it may happen that an ASG's launch template is modified, creating a new
			// template version, but then that new template version is deleted before the node has been terminated.
			// To make it even more of an edge case, the draining function would've had to time out, meaning that
			// the termination would be skipped until the next run.
			// This would cause an instance to be considered as updated, even though it has been drained therefore
			// cordoned (NoSchedule).
			if startedAtValue, ok := updatedNode.Annotations[k8s.RollingUpdateStartedTimestampAnnotationKey]; ok {
				// An updated node should never have k8s.RollingUpdateStartedTimestampAnnotationKey, so this indicates that
				// at one point, this node was considered old compared to the ASG's current LT/LC
				// First, check if there's a NoSchedule taint
				for i, taint := range updatedNode.Spec.Taints {
					if taint.Effect == v1.TaintEffectNoSchedule {
						// There's a taint, but we need to make sure it was added after the rolling update started
						startedAt, err := time.Parse(time.RFC3339, startedAtValue)
						// If the annotation can't be parsed OR the taint was added after the rolling updated started,
						// we need to remove that taint
						if err != nil || taint.TimeAdded.Time.After(startedAt) {
							// Remove the taint
							updatedNode.Spec.Taints = append(updatedNode.Spec.Taints[:i], updatedNode.Spec.Taints[i+1:]...)
							// Remove the annotation
							delete(updatedNode.Annotations, k8s.RollingUpdateStartedTimestampAnnotationKey)
							// Update the node
							err = kubernetesClient.UpdateNode(updatedNode)
							if err != nil {
								log.Printf("[%s] EDGE-0001: Unable to update tainted node %s: %v", *autoScalingGroup.AutoScalingGroupName, updatedNode.Name, err.Error())
							}
							break
						}
					}
				}
			}
		}

		if len(outdatedInstances) == 0 {
			log.Printf("[%s] All instances are up to date", *autoScalingGroup.AutoScalingGroupName)
			continue
		}

		// XXX: this should be configurable (i.e. SLOW_ROLLING_UPDATE)
		if numberOfNonReadyNodesOrInstances != 0 {
			log.Printf("[%s] ASG has %d non-ready updated nodes/instances, waiting until all nodes/instances are ready", *autoScalingGroup.AutoScalingGroupName, numberOfNonReadyNodesOrInstances)
			continue
		}

		for _, outdatedInstance := range outdatedInstances {
			node, err := kubernetesClient.GetNodeByHostName(*outdatedInstance.InstanceId)
			if err != nil {
				log.Printf("[%s][%s] Unable to get outdated node from Kubernetes: %v", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId, err.Error())
				log.Printf("[%s][%s] Skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
				continue
			}

			minutesSinceStarted, minutesSinceDrained, minutesSinceTerminated := getRollingUpdateTimestampsFromNode(node)

			// Check if outdated nodes in k8s have been marked with annotation from aws-eks-asg-rolling-update-handler
			if minutesSinceStarted == -1 {
				log.Printf("[%s][%s] Starting node rollout process", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
				// Annotate the node to persist the fact that the rolling update process has begun
				err := k8s.AnnotateNodeByHostName(kubernetesClient, *outdatedInstance.InstanceId, k8s.RollingUpdateStartedTimestampAnnotationKey, time.Now().Format(time.RFC3339))
				if err != nil {
					log.Printf("[%s][%s] Unable to annotate node: %v", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId, err.Error())
					// XXX: should we really skip here?
					log.Printf("[%s][%s] Skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
					continue
				}
				// TODO: increase desired instance by 1 (to create a new updated instance)

			} else {
				log.Printf("[%s][%s] Node already started rollout process", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
				// check if existing updatedInstances have the capacity to support what's inside this node
				hasEnoughResources := k8s.CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(kubernetesClient, node, updatedReadyNodes)
				if hasEnoughResources {
					log.Printf("[%s][%s] Updated nodes have enough resources available", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
					if minutesSinceDrained == -1 {
						log.Printf("[%s][%s] Draining node", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
						err := kubernetesClient.Drain(node.Name, config.Get().IgnoreDaemonSets, config.Get().DeleteLocalData)
						if err != nil {
							log.Printf("[%s][%s] Ran into error while draining node: %v", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId, err.Error())
							log.Printf("[%s][%s] Skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
							continue
						} else {
							_ = k8s.AnnotateNodeByHostName(kubernetesClient, *outdatedInstance.InstanceId, k8s.RollingUpdateDrainedTimestampAnnotationKey, time.Now().Format(time.RFC3339))
						}
					} else {
						log.Printf("[%s][%s] Node has already been drained, skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
					}
					if minutesSinceTerminated == -1 {
						// Terminate node
						log.Printf("[%s][%s] Terminating node", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
						err = cloud.TerminateEc2Instance(autoScalingService, outdatedInstance)
						if err != nil {
							log.Printf("[%s][%s] Ran into error while terminating node: %v", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId, err.Error())
							continue
						} else {
							_ = k8s.AnnotateNodeByHostName(kubernetesClient, *outdatedInstance.InstanceId, k8s.RollingUpdateTerminatedTimestampAnnotationKey, time.Now().Format(time.RFC3339))
						}
					} else {
						log.Printf("[%s][%s] Node is already in the process of being terminated, skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
						// TODO: check if minutesSinceTerminated > 10. If that happens, then there's clearly a problem, so we should do something about it
					}
					return
				} else {
					log.Printf("[%s][%s] Updated nodes do not have enough resources available, increasing desired count by 1", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
					// TODO: check if desired capacity matches (updatedInstances + outdatedInstances + 1)
					err := cloud.SetAutoScalingGroupDesiredCount(autoScalingService, autoScalingGroup, *autoScalingGroup.DesiredCapacity+1)
					if err != nil {
						log.Printf("[%s][%s] Unable to increase ASG desired size: %v", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId, err.Error())
						log.Printf("[%s][%s] Skipping", *autoScalingGroup.AutoScalingGroupName, *outdatedInstance.InstanceId)
						continue
					}
					return
				}
			}
		}
		// TODO: Check if ASG hit max, and then decide what to do (patience or violence)
	}
}

func getRollingUpdateTimestampsFromNode(node *v1.Node) (minutesSinceStarted int, minutesSinceDrained int, minutesSinceTerminated int) {
	rollingUpdateStartedAt, ok := node.Annotations[k8s.RollingUpdateStartedTimestampAnnotationKey]
	if ok {
		startedAt, err := time.Parse(time.RFC3339, rollingUpdateStartedAt)
		if err == nil {
			minutesSinceStarted = int(time.Since(startedAt).Minutes())
		}
	} else {
		minutesSinceStarted = -1
	}
	drainedAtValue, ok := node.Annotations[k8s.RollingUpdateDrainedTimestampAnnotationKey]
	if ok {
		drainedAt, err := time.Parse(time.RFC3339, drainedAtValue)
		if err == nil {
			minutesSinceDrained = int(time.Since(drainedAt).Minutes())
		}
	} else {
		minutesSinceDrained = -1
	}
	terminatedAtValue, ok := node.Annotations[k8s.RollingUpdateTerminatedTimestampAnnotationKey]
	if ok {
		terminatedAt, err := time.Parse(time.RFC3339, terminatedAtValue)
		if err == nil {
			minutesSinceTerminated = int(time.Since(terminatedAt).Minutes())
		}
	} else {
		minutesSinceTerminated = -1
	}
	return
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
	var (
		oldInstances   []*autoscaling.Instance
		newInstances   []*autoscaling.Instance
		targetTemplate *ec2.LaunchTemplate
		err            error
	)
	switch {
	case targetLaunchTemplate.LaunchTemplateId != nil && *targetLaunchTemplate.LaunchTemplateId != "":
		if targetTemplate, err = cloud.DescribeLaunchTemplateByID(ec2Svc, *targetLaunchTemplate.LaunchTemplateId); err != nil {
			return nil, nil, fmt.Errorf("error retrieving information about launch template %s: %v", *targetLaunchTemplate.LaunchTemplateId, err)
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
	for _, instance := range instances {
		switch {
		case instance.LaunchTemplate == nil:
			fallthrough
		case aws.StringValue(instance.LaunchTemplate.LaunchTemplateName) != aws.StringValue(targetLaunchTemplate.LaunchTemplateName):
			fallthrough
		case aws.StringValue(instance.LaunchTemplate.LaunchTemplateId) != aws.StringValue(targetLaunchTemplate.LaunchTemplateId):
			fallthrough
		case !compareLaunchTemplateVersions(targetTemplate, targetLaunchTemplate, instance.LaunchTemplate):
			oldInstances = append(oldInstances, instance)
		default:
			newInstances = append(newInstances, instance)
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
