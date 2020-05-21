# aws-eks-asg-rolling-update-handler

![build](https://github.com/TwinProduction/aws-eks-asg-rolling-update-handler/workflows/build/badge.svg?branch=master)
[![Docker pulls](https://img.shields.io/docker/pulls/twinproduction/aws-eks-asg-rolling-update-handler.svg)](https://cloud.docker.com/repository/docker/twinproduction/aws-eks-asg-rolling-update-handler)

This application handles rolling upgrades for AWS ASGs for EKS by replacing outdated nodes by new nodes.
Outdated nodes are defined as nodes whose current configuration does not match its ASG's current launch 
template version or launch configuration.

Inspired by aws-asg-roller, this application only has one purpose: Scale down outdated nodes gracefully.

Unlike aws-asg-roller, it will not attempt to control the amount of nodes at all; it will scale up enough new nodes
to move the pods from the old nodes to the new nodes, and then evict the old nodes. 

It will not adjust the desired size back to its initial desired size like aws-asg-roller does, it will simply leave
everything else will be up to cluster-autoscaler.

Note that unlike other solutions, this application actually uses the resources to determine how many instances should 
be spun up before draining the old nodes. This is much better, because simply using the initial number of instances is 
completely useless in the event that the ASG's update on the launch configuration/template is a change of instance type.


## Behavior

On interval, this application:
1. Iterates over each ASG defined by the `AUTO_SCALING_GROUP_NAMES` environment variable
2. Iterates over each instances of each ASGs
3. Checks if there's any instances with an outdated launch template version
4. **If ASG uses MixedInstancesPolicy**, checks if there's any instances with an instance type that isn't part of the list of instance type overrides
5. Checks if there's any instances with an outdated launch configuration
6. If any of the conditions defined in the step 3, 4 or 5 are met for any instance, begin the rolling update process for that instance

The steps of each actions are persisted directly on the old nodes (i.e. when the old node starts rolling out, gets drained, and gets scheduled for termination). Therefore, this application will not run into any issues if it is restarted, rescheduled or stopped at any point in time.


## Usage

| Environment variable | Description | Required | Default |
| --- | --- | --- | --- |
| AUTO_SCALING_GROUP_NAMES | Comma-separated list of ASGs | yes | `""` |
| IGNORE_DAEMON_SETS | Whether to ignore DaemonSets when draining the nodes | no | `true` |
| DELETE_LOCAL_DATA | Whether to delete local data when draining the nodes | no | `true` |
| AWS_REGION | Self-explanatory | no | `us-west-2` |
| ENVIRONMENT | If set to `dev`, will try to create the Kubernetes client using your local kubeconfig. Any other values will use the in-cluster configuration | no | `""` |


## Permissions

To function properly, this application requires the following permissions on AWS:
- autoscaling:DescribeAutoScalingGroups
- autoscaling:DescribeAutoScalingInstances
- autoscaling:DescribeLaunchConfigurations
- autoscaling:SetDesiredCapacity
- autoscaling:TerminateInstanceInAutoScalingGroup
- autoscaling:UpdateAutoScalingGroup
- ec2:DescribeLaunchTemplates
- ec2:DescribeInstances

## Deploying on Kubernetes

```yaml
apiVersion: core/v1
kind: ServiceAccount
metadata:
  name: aws-eks-asg-rolling-update-handler
  namespace: kube-system
  labels:
    k8s-app: aws-eks-asg-rolling-update-handler
  
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: aws-eks-asg-rolling-update-handler
  labels:
    k8s-app: aws-eks-asg-rolling-update-handler
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - "*"
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
      - update
      - patch
  - apiGroups:
      - "*"
    resources:
      - pods/eviction
    verbs:
      - get
      - list
      - create
  - apiGroups:
      - "*"
    resources:
      - pods
    verbs:
      - get
      - list
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: aws-eks-asg-rolling-update-handler
  labels:
    k8s-app: aws-eks-asg-rolling-update-handler
roleRef:
  kind: ClusterRole
  name: aws-eks-asg-rolling-update-handler
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: aws-eks-asg-rolling-update-handler
    namespace: kube-system
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: aws-eks-asg-rolling-update-handler
  namespace: kube-system
  labels:
    k8s-app: aws-eks-asg-rolling-update-handler
spec:
  replicas: 1
  template:
    metadata:
      labels:
        k8s-app: aws-eks-asg-rolling-update-handler
    spec:
      automountServiceAccountToken: true
      serviceAccountName: aws-eks-asg-rolling-update-handler
      restartPolicy: Always
      dnsPolicy: Default
      containers:
        - name: aws-eks-asg-rolling-update-handler
          image: twinproduction/aws-eks-asg-rolling-update-handler
          imagePullPolicy: Always
          env:
            - name: AUTO_SCALING_GROUP_NAMES
              value: "asg-1,asg-2,asg-3" # REPLACE THESE VALUES FOR THE NAMES OF THE ASGs
```


## Developing

To run the application locally, make sure your local kubeconfig file is configured properly (i.e. you can use kubectl).

Once you've done that, set the local environment variable `ENVIRONMENT` to `dev` and `AUTO_SCALING_GROUP_NAMES` 
to a comma-separated list of auto scaling group names.

Your local aws credentials must also be valid (i.e. you can use `awscli`)


## Special thanks

I had originally worked on [deitch/aws-asg-roller](https://github.com/deitch/aws-asg-roller), but due to the numerous conflicts it had with cluster-autoscaler, 
I decided to make a project that heavily relies on cluster-autoscaler rather than simply coexist with it, with a much bigger emphasis on maintaining 
high availability during rolling upgrades.

In any case, this project was inspired by aws-asg-roller and the code for comparing launch template versions also comes from there, hence why this special thanks section exists.