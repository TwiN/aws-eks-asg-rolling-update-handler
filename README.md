# aws-eks-asg-rolling-update-handler

This application handles rolling upgrades for AWS ASGs for EKS by replacing outdated nodes by new nodes.
Outdated nodes are defined as nodes whose current configuration does not match its ASG's current launch 
template version or launch configuration.

Inspired by aws-asg-roller, this application only has one purpose: Scale down outdated nodes gracefully.

Unlike aws-asg-roller, it will not attempt to control the amount of nodes at all; it will scale up enough new nodes
to move the pods from the old nodes to the new nodes, and then evict the old nodes. 

Everything else will be up to cluster-autoscaler.
