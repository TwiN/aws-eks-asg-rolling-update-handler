# aws-eks-asg-rolling-update-handler

This application handles rolling upgrades for AWS ASGs for EKS by replacing outdated nodes by new nodes.
Outdated nodes are defined as nodes whose current configuration does not match its ASG's current launch 
template version or launch configuration.

Inspired by aws-asg-roller, this application only has one purpose: Scale down outdated nodes gracefully.

Unlike aws-asg-roller, it will not attempt to control the amount of nodes at all; it will scale up enough new nodes
to move the pods from the old nodes to the new nodes, and then evict the old nodes. 

Everything else will be up to cluster-autoscaler.


## Usage

| Environment variable | Description | Required | Default |
| --- | --- | --- | --- |
| AUTO_SCALING_GROUP_NAMES | Comma-separated list of ASGs | yes | `""` |
| IGNORE_DAEMON_SETS | Whether to ignore DaemonSets when draining the nodes | no | `true` |
| DELETE_LOCAL_DATA | Whether to delete local data when draining the nodes | no | `true` |
| AWS_REGION | Self-explanatory | no | `us-west-2` |
| ENVIRONMENT | If set to `dev`, will try to create the Kubernetes client using your local kubeconfig. Any other values will use the in-cluster configuration | no | `""` |


### Running locally 

To run the application locally, make sure your local kubeconfig file is configured properly (i.e. you can use kubectl).

Once you've done that, set the local environment variable `ENVIRONMENT` to `dev` and `AUTO_SCALING_GROUP_NAMES` 
to a comma-separated list of auto scaling group names.

Your local aws credentials must also be valid (i.e. you can use `awscli`)
