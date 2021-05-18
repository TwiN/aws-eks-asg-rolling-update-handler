package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var cfg *config

const (
	EnvEnvironment           = "ENVIRONMENT"
	EnvDebug                 = "DEBUG"
	EnvIgnoreDaemonSets      = "IGNORE_DAEMON_SETS"
	EnvDeleteLocalData       = "DELETE_LOCAL_DATA"
	EnvClusterName           = "CLUSTER_NAME"
	EnvAutoScalingGroupNames = "AUTO_SCALING_GROUP_NAMES"
	EnvAwsRegion             = "AWS_REGION"
)

type config struct {
	// Optional
	Environment string

	// Defaults to false
	Debug bool

	// Required if ClusterName not provided
	AutoScalingGroupNames []string

	// Required if AutoScalingGroupNames not provided
	ClusterName string

	// Defaults to us-west-2
	AwsRegion string

	// Defaults to true
	IgnoreDaemonSets bool

	// Defaults to true
	DeleteLocalData bool
}

// Initialize is used to initialize the application's configuration
func Initialize() error {
	cfg = &config{
		Environment: strings.ToLower(os.Getenv(EnvEnvironment)),
		Debug:       strings.ToLower(os.Getenv(EnvDebug)) == "true",
	}
	if autoScalingGroupNames := os.Getenv(EnvAutoScalingGroupNames); len(autoScalingGroupNames) > 0 {
		cfg.AutoScalingGroupNames = strings.Split(strings.TrimSpace(autoScalingGroupNames), ",")
	} else if clusterName := os.Getenv(EnvClusterName); len(clusterName) > 0 {
		cfg.ClusterName = clusterName
	} else {
		return fmt.Errorf("environment variables '%s' or '%s' are not set", EnvAutoScalingGroupNames, EnvClusterName)
	}
	if ignoreDaemonSets := strings.ToLower(os.Getenv(EnvIgnoreDaemonSets)); len(ignoreDaemonSets) == 0 || ignoreDaemonSets == "true" {
		cfg.IgnoreDaemonSets = true
	}
	if deleteLocalData := strings.ToLower(os.Getenv(EnvDeleteLocalData)); len(deleteLocalData) == 0 || deleteLocalData == "true" {
		cfg.DeleteLocalData = true
	}
	if awsRegion := strings.ToLower(os.Getenv(EnvAwsRegion)); len(awsRegion) == 0 {
		log.Printf("Environment variable '%s' not specified, defaulting to us-west-2", EnvAwsRegion)
		cfg.AwsRegion = "us-west-2"
	} else {
		cfg.AwsRegion = awsRegion
	}
	return nil
}

// Set sets the application's configuration and is intended to be used for testing purposes.
// See Initialize() for production
func Set(autoScalingGroupNames []string, ignoreDaemonSets, deleteLocalData bool) {
	cfg = &config{
		AutoScalingGroupNames: autoScalingGroupNames,
		IgnoreDaemonSets:      ignoreDaemonSets,
		DeleteLocalData:       deleteLocalData,
	}
}

func Get() *config {
	if cfg == nil {
		log.Println("Config wasn't initialized prior to being called. Assuming this is a test.")
		cfg = &config{}
	}
	return cfg
}
