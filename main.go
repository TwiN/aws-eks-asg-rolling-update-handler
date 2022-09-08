package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/TwiN/aws-eks-asg-rolling-update-handler/cloud"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/config"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/k8s"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	v1 "k8s.io/api/core/v1"
)

const (
	MaximumFailedExecutionBeforePanic = 10 // Maximum number of allowed failed executions before panicking
)

var (
	ErrTimedOut = errors.New("execution timed out")

	executionFailedCounter = 0
)

func main() {
	err := config.Initialize()
	if err != nil {
		log.Fatalf("Unable to initialize configuration: %s", err.Error())
	}
	if config.Get().Metrics {
		go metrics.Server.Listen(config.Get().MetricsPort)
	}
	ec2Service, autoScalingService, err := cloud.GetServices(config.Get().AwsRegion)
	if err != nil {
		log.Fatalf("Unable to create AWS services: %s", err.Error())
	}
	for {
		start := time.Now()
		if err := run(ec2Service, autoScalingService); err != nil {
			log.Printf("Error during execution: %s", err.Error())
			metrics.Server.Errors.Inc()
			executionFailedCounter++
			if executionFailedCounter > MaximumFailedExecutionBeforePanic {
				panic(fmt.Errorf("execution failed %d times: %v", executionFailedCounter, err))
			}
		} else if executionFailedCounter > 0 {
			log.Printf("Execution was successful after %d failed attempts, resetting counter to 0", executionFailedCounter)
			executionFailedCounter = 0
		}
		log.Printf("Execution took %dms, sleeping for %s", time.Since(start).Milliseconds(), config.Get().ExecutionInterval)
		time.Sleep(config.Get().ExecutionInterval)
	}
}

func run(ec2Service ec2iface.EC2API, autoScalingService autoscalingiface.AutoScalingAPI) error {
	log.Println("Starting execution")
	cfg := config.Get()
	client, err := k8s.CreateClientSet()
	if err != nil {
		return errors.New("unable to create Kubernetes client: " + err.Error())
	}
	kubernetesClient := k8s.NewClient(client)
	if cfg.Debug {
		log.Println("Created Kubernetes Client successfully")
	}

	var autoScalingGroups []*autoscaling.Group
	if len(cfg.AutodiscoveryTags) > 0 {
		autoScalingGroups, err = cloud.DescribeEnabledAutoScalingGroupsByTags(autoScalingService, cfg.AutodiscoveryTags)
	} else {
		autoScalingGroups, err = cloud.DescribeAutoScalingGroupsByNames(autoScalingService, cfg.AutoScalingGroupNames)
	}
	if err != nil {
		return errors.New("unable to describe AutoScalingGroups: " + err.Error())
	}
	if cfg.Debug {
		log.Println("Described AutoScalingGroups successfully")
	}
	return HandleRollingUpgrade(kubernetesClient, ec2Service, autoScalingService, autoScalingGroups)
}

// HandleRollingUpgrade handles rolling upgrades.
//
// Returns an error if an execution lasts for longer than ExecutionTimeout
func HandleRollingUpgrade(client k8s.ClientAPI, ec2Service ec2iface.EC2API, autoScalingService autoscalingiface.AutoScalingAPI, autoScalingGroups []*autoscaling.Group) error {
	metrics.Server.NodeGroups.WithLabelValues().Set(float64(len(autoScalingGroups)))
	timeout := make(chan bool, 1)
	result := make(chan bool, 1)
	go func() {
		time.Sleep(config.Get().ExecutionTimeout)
		timeout <- true
	}()
	go func() {
		result <- DoHandleRollingUpgrade(client, ec2Service, autoScalingService, autoScalingGroups)
	}()
	select {
	case <-timeout:
		return ErrTimedOut
	case <-result:
		return nil
	}
}

// DoHandleRollingUpgrade handles rolling upgrades by iterating over every single AutoScalingGroups' outdated
// instances
func DoHandleRollingUpgrade(client k8s.ClientAPI, ec2Service ec2iface.EC2API, autoScalingService autoscalingiface.AutoScalingAPI, autoScalingGroups []*autoscaling.Group) bool {
	for _, autoScalingGroup := range autoScalingGroups {
		outdatedInstances, updatedInstances, err := SeparateOutdatedFromUpdatedInstances(autoScalingGroup, ec2Service)
		if err != nil {
			metrics.Server.Errors.Inc()
			log.Printf("[%s] Skipping because unable to separate outdated instances from updated instances: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), err.Error())
			continue
		}
		fmt.Printf("[%s] Found %d outdated instances and %d updated instances\n", aws.StringValue(autoScalingGroup.AutoScalingGroupName), len(outdatedInstances), len(updatedInstances))
		metrics.Server.UpdatedNodes.WithLabelValues(aws.StringValue(autoScalingGroup.AutoScalingGroupName)).Set(float64(len(updatedInstances)))
		metrics.Server.OutdatedNodes.WithLabelValues(aws.StringValue(autoScalingGroup.AutoScalingGroupName)).Set(float64(len(outdatedInstances)))
		if config.Get().Debug {
			log.Printf("[%s] outdatedInstances: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), outdatedInstances)
			log.Printf("[%s] updatedInstances: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), updatedInstances)
		}
		// Get the updated and ready nodes from the list of updated instances
		// This will be used to determine if the desired number of updated instances need to scale up or not
		// We also use this to clean up, if necessary
		updatedReadyNodes, numberOfNonReadyNodesOrInstances := getReadyNodesAndNumberOfNonReadyNodesOrInstances(client, updatedInstances, autoScalingGroup)
		if len(outdatedInstances) == 0 {
			log.Printf("[%s] All instances are up to date", aws.StringValue(autoScalingGroup.AutoScalingGroupName))
			continue
		} else {
			log.Printf("[%s] outdated=%d; updated=%d; updatedAndReady=%d; asgCurrent=%d; asgDesired=%d; asgMax=%d", aws.StringValue(autoScalingGroup.AutoScalingGroupName), len(outdatedInstances), len(updatedInstances), len(updatedReadyNodes), len(autoScalingGroup.Instances), aws.Int64Value(autoScalingGroup.DesiredCapacity), aws.Int64Value(autoScalingGroup.MaxSize))
		}
		if int64(len(autoScalingGroup.Instances)) < aws.Int64Value(autoScalingGroup.DesiredCapacity) {
			log.Printf("[%s] Skipping because ASG has a desired capacity of %d, but only has %d instances", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.Int64Value(autoScalingGroup.DesiredCapacity), len(autoScalingGroup.Instances))
			continue
		}
		if numberOfNonReadyNodesOrInstances != 0 {
			log.Printf("[%s] ASG has %d non-ready updated nodes/instances, waiting until all nodes/instances are ready", aws.StringValue(autoScalingGroup.AutoScalingGroupName), numberOfNonReadyNodesOrInstances)
			continue
		}
		for _, outdatedInstance := range outdatedInstances {
			node, err := client.GetNodeByAutoScalingInstance(outdatedInstance)
			if err != nil {
				log.Printf("[%s][%s] Skipping because unable to get outdated node from Kubernetes: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), err.Error())
				continue
			}
			minutesSinceStarted, minutesSinceDrained, minutesSinceTerminated := getRollingUpdateTimestampsFromNode(node)
			// Check if outdated nodes in k8s have been marked with annotation from aws-eks-asg-rolling-update-handler
			if minutesSinceStarted == -1 {
				log.Printf("[%s][%s] Starting node rollout process", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
				// Annotate the node to persist the fact that the rolling update process has begun
				err := k8s.AnnotateNodeByAutoScalingInstance(client, outdatedInstance, k8s.AnnotationRollingUpdateStartedTimestamp, time.Now().Format(time.RFC3339))
				if err != nil {
					log.Printf("[%s][%s] Skipping because unable to annotate node: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), err.Error())
					continue
				}
			} else {
				log.Printf("[%s][%s] Node already started rollout process", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
				// check if existing updatedInstances have the capacity to support what's inside this node
				hasEnoughResources := k8s.CheckIfNodeHasEnoughResourcesToTransferAllPodsInNodes(client, node, updatedReadyNodes)
				if hasEnoughResources {
					log.Printf("[%s][%s] Updated nodes have enough resources available", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
					if minutesSinceDrained == -1 {
						log.Printf("[%s][%s] Draining node", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
						err := client.Drain(node.Name, config.Get().IgnoreDaemonSets, config.Get().DeleteEmptyDirData, config.Get().PodTerminationGracePeriod)
						if err != nil {
							metrics.Server.Errors.Inc()
							log.Printf("[%s][%s] Skipping because ran into error while draining node: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), err.Error())
							continue
						} else {
							metrics.Server.DrainedNodes.WithLabelValues(aws.StringValue(autoScalingGroup.AutoScalingGroupName)).Inc()
							// Only annotate if no error was encountered
							_ = k8s.AnnotateNodeByAutoScalingInstance(client, outdatedInstance, k8s.AnnotationRollingUpdateDrainedTimestamp, time.Now().Format(time.RFC3339))
						}
					} else {
						log.Printf("[%s][%s] Node has already been drained %d minutes ago, skipping", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), minutesSinceDrained)
					}
					if minutesSinceTerminated == -1 {
						// Terminate node
						log.Printf("[%s][%s] Terminating node", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
						shouldDecrementDesiredCapacity := aws.Int64Value(autoScalingGroup.DesiredCapacity) != aws.Int64Value(autoScalingGroup.MinSize)
						err = cloud.TerminateEc2Instance(autoScalingService, outdatedInstance, shouldDecrementDesiredCapacity)
						if err != nil {
							metrics.Server.Errors.Inc()
							log.Printf("[%s][%s] Ran into error while terminating node: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), err.Error())
							continue
						} else {
							metrics.Server.ScaledDownNodes.WithLabelValues(aws.StringValue(autoScalingGroup.AutoScalingGroupName)).Inc()
							// Only annotate if no error was encountered
							_ = k8s.AnnotateNodeByAutoScalingInstance(client, outdatedInstance, k8s.AnnotationRollingUpdateTerminatedTimestamp, time.Now().Format(time.RFC3339))
						}
					} else {
						log.Printf("[%s][%s] Node is already in the process of being terminated since %d minutes ago, skipping", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), minutesSinceTerminated)
						// TODO: check if minutesSinceTerminated > 10. If that happens, then there's clearly a problem, so we should do something about it
						// The node has already been terminated, there's nothing to do here, continue to the next one
						continue
					}
					// If this code is reached, it means that the current node has been successfully drained and
					// scheduled for termination.
					// As a result, we return here to make sure that multiple old instances didn't use the same updated
					// instances to calculate resources available
					log.Printf("[%s][%s] Node has been drained and scheduled for termination successfully", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
					return true
				} else {
					// Don't increase the ASG if the node has already been drained or scheduled for termination
					if minutesSinceDrained != -1 || minutesSinceTerminated != -1 {
						continue
					}
					log.Printf("[%s][%s] Updated nodes do not have enough resources available, increasing desired count by 1", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
					err := cloud.SetAutoScalingGroupDesiredCount(autoScalingService, autoScalingGroup, aws.Int64Value(autoScalingGroup.DesiredCapacity)+1)
					if err != nil {
						log.Printf("[%s][%s] Unable to increase ASG desired size: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId), err.Error())
						log.Printf("[%s][%s] Skipping", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(outdatedInstance.InstanceId))
						continue
					} else {
						metrics.Server.ScaledUpNodes.WithLabelValues(aws.StringValue(autoScalingGroup.AutoScalingGroupName)).Inc()
						// ASG was scaled up already, stop iterating over outdated instances in current ASG so we can
						// move on to the next ASG
						break
					}
				}
			}
		}
	}
	return true
}

func getReadyNodesAndNumberOfNonReadyNodesOrInstances(client k8s.ClientAPI, updatedInstances []*autoscaling.Instance, autoScalingGroup *autoscaling.Group) ([]*v1.Node, int) {
	var updatedReadyNodes []*v1.Node
	numberOfNonReadyNodesOrInstances := 0
	for _, updatedInstance := range updatedInstances {
		if aws.StringValue(updatedInstance.LifecycleState) != "InService" {
			numberOfNonReadyNodesOrInstances++
			log.Printf("[%s][%s] Skipping because instance is not in LifecycleState 'InService', but is in '%s' instead", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(updatedInstance.InstanceId), aws.StringValue(updatedInstance.LifecycleState))
			continue
		}
		updatedNode, err := client.GetNodeByAutoScalingInstance(updatedInstance)
		if err != nil {
			numberOfNonReadyNodesOrInstances++
			log.Printf("[%s][%s] Skipping because unable to get updated node from Kubernetes: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(updatedInstance.InstanceId), err.Error())
			continue
		}
		// Check if Kubelet is ready to accept pods on that node
		conditions := updatedNode.Status.Conditions
		if len(conditions) == 0 {
			log.Printf("[%s][%s] For some magical reason, %s doesn't have any conditions, therefore it is impossible to determine whether the node is ready to accept new pods or not", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(updatedInstance.InstanceId), updatedNode.Name)
			numberOfNonReadyNodesOrInstances++
		} else if kubeletCondition := conditions[len(conditions)-1]; kubeletCondition.Type == v1.NodeReady {
			if kubeletCondition.Status == v1.ConditionTrue {
				updatedReadyNodes = append(updatedReadyNodes, updatedNode)
			} else {
				log.Printf("[%s][%s] Skipping because kubelet condition %s is reporting as %s", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(updatedInstance.InstanceId), kubeletCondition.Type, kubeletCondition.Status)
				numberOfNonReadyNodesOrInstances++
			}
		} else {
			log.Printf("[%s][%s] Skipping because expected kubelet on node to have condition %s with value %s, but it didn't", aws.StringValue(autoScalingGroup.AutoScalingGroupName), aws.StringValue(updatedInstance.InstanceId), v1.NodeReady, v1.ConditionTrue)
			numberOfNonReadyNodesOrInstances++
		}

		// Cleaning up
		// This is an edge case, but it may happen that an ASG's launch template is modified, creating a new
		// template version, but then that new template version is deleted before the node has been terminated.
		// To make it even more of an edge case, the draining function would've had to time out, meaning that
		// the termination would be skipped until the next run.
		// This would cause an instance to be considered as updated, even though it has been drained therefore
		// cordoned (NoSchedule).
		if startedAtValue, ok := updatedNode.Annotations[k8s.AnnotationRollingUpdateStartedTimestamp]; ok {
			// An updated node should never have k8s.AnnotationRollingUpdateStartedTimestamp, so this indicates that
			// at one point, this node was considered old compared to the ASG's current LT/LC
			// First, check if there's a NoSchedule taint
			for i, taint := range updatedNode.Spec.Taints {
				if taint.Effect == v1.TaintEffectNoSchedule {
					// There's a taint, but we need to make sure it was added after the rolling update started
					startedAt, err := time.Parse(time.RFC3339, startedAtValue)
					// If the annotation can't be parsed OR the taint was added after the rolling updated started,
					// we need to remove that taint
					if err != nil || taint.TimeAdded.Time.After(startedAt) {
						log.Printf("[%s] EDGE-0001: Attempting to remove taint from updated node %s", aws.StringValue(autoScalingGroup.AutoScalingGroupName), updatedNode.Name)
						// Remove the taint
						updatedNode.Spec.Taints = append(updatedNode.Spec.Taints[:i], updatedNode.Spec.Taints[i+1:]...)
						// Remove the annotation
						delete(updatedNode.Annotations, k8s.AnnotationRollingUpdateStartedTimestamp)
						// Update the node
						err = client.UpdateNode(updatedNode)
						if err != nil {
							log.Printf("[%s] EDGE-0001: Unable to update tainted node %s: %v", aws.StringValue(autoScalingGroup.AutoScalingGroupName), updatedNode.Name, err.Error())
						}
						break
					}
				}
			}
		}
	}
	return updatedReadyNodes, numberOfNonReadyNodesOrInstances
}

func getRollingUpdateTimestampsFromNode(node *v1.Node) (minutesSinceStarted int, minutesSinceDrained int, minutesSinceTerminated int) {
	rollingUpdateStartedAt, ok := node.Annotations[k8s.AnnotationRollingUpdateStartedTimestamp]
	if ok {
		startedAt, err := time.Parse(time.RFC3339, rollingUpdateStartedAt)
		if err == nil {
			minutesSinceStarted = int(time.Since(startedAt).Minutes())
		}
	} else {
		minutesSinceStarted = -1
	}
	drainedAtValue, ok := node.Annotations[k8s.AnnotationRollingUpdateDrainedTimestamp]
	if ok {
		drainedAt, err := time.Parse(time.RFC3339, drainedAtValue)
		if err == nil {
			minutesSinceDrained = int(time.Since(drainedAt).Minutes())
		}
	} else {
		minutesSinceDrained = -1
	}
	terminatedAtValue, ok := node.Annotations[k8s.AnnotationRollingUpdateTerminatedTimestamp]
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

// SeparateOutdatedFromUpdatedInstances splits a list of instances into a list of outdated
// instances and a list of updated instances.
func SeparateOutdatedFromUpdatedInstances(asg *autoscaling.Group, ec2Svc ec2iface.EC2API) ([]*autoscaling.Instance, []*autoscaling.Instance, error) {
	if config.Get().Debug {
		log.Printf("[%s] Separating outdated from updated instances", aws.StringValue(asg.AutoScalingGroupName))
	}
	targetLaunchConfiguration := asg.LaunchConfigurationName
	targetLaunchTemplate := asg.LaunchTemplate
	var targetLaunchTemplateOverrides []*autoscaling.LaunchTemplateOverrides
	if targetLaunchTemplate == nil && asg.MixedInstancesPolicy != nil && asg.MixedInstancesPolicy.LaunchTemplate != nil {
		if config.Get().Debug {
			log.Printf("[%s] using mixed instances policy launch template", aws.StringValue(asg.AutoScalingGroupName))
		}
		targetLaunchTemplate = asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification
		targetLaunchTemplateOverrides = asg.MixedInstancesPolicy.LaunchTemplate.Overrides
	}
	if targetLaunchTemplate != nil {
		return SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(aws.StringValue(asg.AutoScalingGroupName), targetLaunchTemplate, targetLaunchTemplateOverrides, asg.Instances, ec2Svc)
	} else if targetLaunchConfiguration != nil {
		return SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration(targetLaunchConfiguration, asg.Instances)
	}
	return nil, nil, errors.New("AutoScalingGroup has neither launch template nor launch configuration")
}

// SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate separates a list of instances into a list of outdated
// instances and a list of updated instances.
func SeparateOutdatedFromUpdatedInstancesUsingLaunchTemplate(asgName string, targetLaunchTemplate *autoscaling.LaunchTemplateSpecification, overrides []*autoscaling.LaunchTemplateOverrides, instances []*autoscaling.Instance, ec2Svc ec2iface.EC2API) ([]*autoscaling.Instance, []*autoscaling.Instance, error) {
	var (
		oldInstances   []*autoscaling.Instance
		newInstances   []*autoscaling.Instance
		targetTemplate *ec2.LaunchTemplate
		err            error
	)
	switch {
	case targetLaunchTemplate.LaunchTemplateId != nil && aws.StringValue(targetLaunchTemplate.LaunchTemplateId) != "":
		if targetTemplate, err = cloud.DescribeLaunchTemplateByID(ec2Svc, aws.StringValue(targetLaunchTemplate.LaunchTemplateId)); err != nil {
			return nil, nil, fmt.Errorf("error retrieving information about launch template %s: %v", aws.StringValue(targetLaunchTemplate.LaunchTemplateId), err)
		}
	case targetLaunchTemplate.LaunchTemplateName != nil && aws.StringValue(targetLaunchTemplate.LaunchTemplateName) != "":
		if targetTemplate, err = cloud.DescribeLaunchTemplateByName(ec2Svc, aws.StringValue(targetLaunchTemplate.LaunchTemplateName)); err != nil {
			return nil, nil, fmt.Errorf("error retrieving information about launch template name %s: %v", aws.StringValue(targetLaunchTemplate.LaunchTemplateName), err)
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
		if isInstanceTypePartOfLaunchTemplateOverrides(overrides, instance.InstanceType) {
			var (
				overrideTargetTemplate       *ec2.LaunchTemplate
				overrideTargetLaunchTemplate *autoscaling.LaunchTemplateSpecification
			)
			for _, override := range overrides {
				if aws.StringValue(override.InstanceType) == aws.StringValue(instance.InstanceType) && override.LaunchTemplateSpecification != nil {
					if overrideTargetTemplate, err = cloud.DescribeLaunchTemplateByName(ec2Svc, aws.StringValue(override.LaunchTemplateSpecification.LaunchTemplateName)); err != nil {
						log.Printf("[%s][%s] Unable to retrieve information for launch template with name '%s': %v", asgName, aws.StringValue(instance.InstanceId), aws.StringValue(override.LaunchTemplateSpecification.LaunchTemplateName), err)
					}
					overrideTargetLaunchTemplate = override.LaunchTemplateSpecification
				}
			}
			if overrideTargetTemplate != nil && overrideTargetLaunchTemplate != nil {
				targetTemplate = overrideTargetTemplate
				targetLaunchTemplate = overrideTargetLaunchTemplate
			}
		}
		switch {
		case instance.LaunchTemplate == nil:
			fallthrough
		case aws.StringValue(instance.LaunchTemplate.LaunchTemplateName) != aws.StringValue(targetLaunchTemplate.LaunchTemplateName):
			fallthrough
		case aws.StringValue(instance.LaunchTemplate.LaunchTemplateId) != aws.StringValue(targetLaunchTemplate.LaunchTemplateId):
			fallthrough
		case !compareLaunchTemplateVersions(targetTemplate, targetLaunchTemplate, instance.LaunchTemplate):
			fallthrough
		case overrides != nil && len(overrides) > 0 && !isInstanceTypePartOfLaunchTemplateOverrides(overrides, instance.InstanceType):
			oldInstances = append(oldInstances, instance)
		default:
			newInstances = append(newInstances, instance)
		}
	}
	return oldInstances, newInstances, nil
}

func isInstanceTypePartOfLaunchTemplateOverrides(overrides []*autoscaling.LaunchTemplateOverrides, instanceType *string) bool {
	for _, override := range overrides {
		if aws.StringValue(override.InstanceType) == aws.StringValue(instanceType) {
			return true
		}
	}
	return false
}

// SeparateOutdatedFromUpdatedInstancesUsingLaunchConfiguration separates a list of instances into a list of outdated
// instances and a list of updated instances.
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
	switch aws.StringValue(lt1.Version) {
	case "$Default":
		lt1version = fmt.Sprintf("%d", aws.Int64Value(targetTemplate.DefaultVersionNumber))
	case "$Latest":
		lt1version = fmt.Sprintf("%d", aws.Int64Value(targetTemplate.LatestVersionNumber))
	default:
		lt1version = aws.StringValue(lt1.Version)
	}
	switch aws.StringValue(lt2.Version) {
	case "$Default":
		lt2version = fmt.Sprintf("%d", aws.Int64Value(targetTemplate.DefaultVersionNumber))
	case "$Latest":
		lt2version = fmt.Sprintf("%d", aws.Int64Value(targetTemplate.LatestVersionNumber))
	default:
		lt2version = aws.StringValue(lt2.Version)
	}
	return lt1version == lt2version
}
