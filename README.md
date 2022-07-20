# aws-eks-asg-rolling-update-handler

![test](https://github.com/TwiN/aws-eks-asg-rolling-update-handler/workflows/test/badge.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/TwiN/aws-eks-asg-rolling-update-handler)](https://goreportcard.com/report/github.com/TwiN/aws-eks-asg-rolling-update-handler)
[![Docker pulls](https://img.shields.io/docker/pulls/twinproduction/aws-eks-asg-rolling-update-handler.svg)](https://cloud.docker.com/repository/docker/twinproduction/aws-eks-asg-rolling-update-handler)

This application handles rolling upgrades for AWS ASGs for EKS by replacing outdated nodes by new nodes.
Outdated nodes are defined as nodes whose current configuration does not match its ASG's current launch 
template version or launch configuration.

Inspired by aws-asg-roller, this application only has one purpose: Scale down outdated nodes gracefully.

Unlike aws-asg-roller, it will not attempt to control the amount of nodes at all; it will scale up enough new nodes
to move the pods from the old nodes to the new nodes, and then evict the old nodes. 

It will not adjust the desired size back to its initial desired size like aws-asg-roller does, it will simply leave
everything else up to cluster-autoscaler.

Note that unlike other solutions, this application actually uses the resources to determine how many instances should 
be spun up before draining the old nodes. This is much better, because simply using the initial number of instances is 
completely useless in the event that the ASG's update on the launch configuration/template is a change of instance type.


## Behavior

On interval, this application:
1. Iterates over each ASG discovered by the `CLUSTER_NAME`, `AUTODISCOVERY_TAGS` environment variables or the ones defined in the `AUTO_SCALING_GROUP_NAMES` environment variable, in that order.
2. Iterates over each instance of each ASG
3. Checks if there's any instance with an outdated launch template version
4. **If ASG uses MixedInstancesPolicy**, checks if there's any instances with an instance type that isn't part of the list of instance type overrides
5. Checks if there's any instance with an outdated launch configuration
6. If any of the conditions defined in the step 3, 4 or 5 are met for any instance, begin the rolling update process for that instance

The steps of each action are persisted directly on the old nodes via annotations (i.e. when the old node starts rolling out, gets drained, and gets scheduled for termination).
Therefore, this application will not run into any issues if it is restarted, rescheduled or stopped at any point in time.


**NOTE**: Ensure that your PodDisruptionBudgets - if you have any - are properly configured. This usually means having at least 1 allowed disruption at all time (i.e. at least `minAvailable: 1` with at least 2 replicas OR `maxUnavailable: 1`)


## Usage

| Environment variable     | Description                                                                                                                                                                                                            | Required | Default     |
|:-------------------------|:-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:---------|:------------|
| CLUSTER_NAME             | Name of the eks-cluster, used in place of `AUTODISCOVERRY_TAGS` and `AUTO_SCALING_GROUP_NAMES`. Checks for `k8s.io/cluster-autoscaler/<CLUSTER_NAME>: owned` and `k8s.io/cluster-autoscaler/enabled: true` tags on ASG | yes      | `""`        |
| AUTODISCOVERY_TAGS       | Comma separated key value string with tags to autodiscover ASGs, used in place of `CLUSTER_NAME` and `AUTO_SCALING_GROUP_NAMES`.                                                                                       | yes      | `""`        |
| AUTO_SCALING_GROUP_NAMES | Comma-separated list of ASGs, CLUSTER_NAME takes priority.                                                                                                                                                             | yes      | `""`        |
| IGNORE_DAEMON_SETS       | Whether to ignore DaemonSets when draining the nodes                                                                                                                                                                   | no       | `true`      |
| DELETE_EMPTY_DIR_DATA    | Whether to delete empty dir data when draining the nodes                                                                                                                                                               | no       | `true`      |
| AWS_REGION               | Self-explanatory                                                                                                                                                                                                       | no       | `us-west-2` |
| ENVIRONMENT              | If set to `dev`, will try to create the Kubernetes client using your local kubeconfig. Any other values will use the in-cluster configuration                                                                          | no       | `""`        |
| EXECUTION_INTERVAL       | Duration to sleep between each execution in seconds                                                                                                                                                                    | no       | `20`        |
| EXECUTION_TIMEOUT        | Maximum execution duration before timing out in seconds                                                                                                                                                                | no       | `900`       |
| METRICS_PORT             | Port to bind metrics server to                                                                                                                                                                                         | no       | `8080`      |
| METRICS                  | Expose metrics in Promtheus format at `:${METRICS_PORT}/metrics`                                                                                                                                                       | no       | `""`        | 


## Metrics

| Metric name                                | Metric type | Labels       | Description                           |
|--------------------------------------------|-------------|--------------|---------------------------------------|
| rolling_update_handler_node_groups         | Gauge       |              | Node groups managed by the handler    |
| rolling_update_handler_outdated_nodes      | Gauge       | `node_group` | The number of outdated nodes          |
| rolling_update_handler_updated_nodes       | Gauge       | `node_group` | The number of updated nodes           |
| rolling_update_handler_scaled_up_nodes     | Counter     | `node_group` | The total number of nodes scaled up   |
| rolling_update_handler_scaled_down_nodes   | Counter     | `node_group` | The total number of nodes scaled down |
| rolling_update_handler_drained_nodes_total | Counter     | `node_group` | The total number of drained nodes     |
| rolling_update_handler_errors              | Counter     |              | The total number of errors            |


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
apiVersion: v1
kind: ServiceAccount
metadata:
  name: aws-eks-asg-rolling-update-handler
  namespace: kube-system
  labels:
    app: aws-eks-asg-rolling-update-handler
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: aws-eks-asg-rolling-update-handler
  labels:
    app: aws-eks-asg-rolling-update-handler
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
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: aws-eks-asg-rolling-update-handler
  labels:
    app: aws-eks-asg-rolling-update-handler
roleRef:
  kind: ClusterRole
  name: aws-eks-asg-rolling-update-handler
  apiGroup: rbac.authorization.k8s.io
subjects:
  - kind: ServiceAccount
    name: aws-eks-asg-rolling-update-handler
    namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aws-eks-asg-rolling-update-handler
  namespace: kube-system
  labels:
    app: aws-eks-asg-rolling-update-handler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: aws-eks-asg-rolling-update-handler
  template:
    metadata:
      labels:
        app: aws-eks-asg-rolling-update-handler
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


## Deploying with Helm

For the chart associated to this project, see [TwiN/helm-charts](https://github.com/TwiN/helm-charts):
```sh
helm repo add twin https://twin.github.io/helm-charts
helm repo update
helm install aws-eks-asg-rolling-update-handler twin/aws-eks-asg-rolling-update-handler
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
