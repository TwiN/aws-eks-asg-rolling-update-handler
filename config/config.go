package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var cfg *config

const (
	EnvEnvironment           = "ENVIRONMENT"
	EnvDebug                 = "DEBUG"
	EnvIgnoreDaemonSets      = "IGNORE_DAEMON_SETS"
	EnvDeleteLocalData       = "DELETE_LOCAL_DATA"
	EnvClusterName           = "CLUSTER_NAME"
	EnvAutodiscoveryTags     = "AUTODISCOVERY_TAGS"
	EnvAutoScalingGroupNames = "AUTO_SCALING_GROUP_NAMES"
	EnvAwsRegion             = "AWS_REGION"
	EnvExecutionInterval     = "EXECUTION_INTERVAL"
	EnvExecutionTimeout      = "EXECUTION_TIMEOUT"
	EnvMetrics               = "METRICS"
	EnvMetricsPort           = "METRICS_PORT"
)

type config struct {
	Environment           string        // Optional
	Debug                 bool          // Defaults to false
	AutoScalingGroupNames []string      // Required if ClusterName not provided
	AutodiscoveryTags     string        // Required if AutoScalingGroupNames not provided
	AwsRegion             string        // Defaults to us-west-2
	IgnoreDaemonSets      bool          // Defaults to true
	DeleteLocalData       bool          // Defaults to true
	ExecutionInterval     time.Duration // Defaults to 20s
	ExecutionTimeout      time.Duration // Defaults to 900s
	Metrics               bool          // Defaults to false
	MetricsPort           int           // Defaults to 8080
}

// Initialize is used to initialize the application's configuration
func Initialize() error {
	cfg = &config{
		Environment: strings.ToLower(os.Getenv(EnvEnvironment)),
		Debug:       strings.ToLower(os.Getenv(EnvDebug)) == "true",
	}
	if clusterName := os.Getenv(EnvClusterName); len(clusterName) > 0 {
		cfg.AutodiscoveryTags = fmt.Sprintf("k8s.io/cluster-autoscaler/%s=owned,k8s.io/cluster-autoscaler/enabled=true", clusterName)
	} else if autodiscoveryTags := os.Getenv(EnvAutodiscoveryTags); len(autodiscoveryTags) > 0 {
		cfg.AutodiscoveryTags = autodiscoveryTags
	} else if autoScalingGroupNames := os.Getenv(EnvAutoScalingGroupNames); len(autoScalingGroupNames) > 0 {
		cfg.AutoScalingGroupNames = strings.Split(strings.TrimSpace(autoScalingGroupNames), ",")
	} else {
		return fmt.Errorf("environment variables '%s', '%s' or '%s' are not set", EnvAutoScalingGroupNames, EnvClusterName, EnvAutodiscoveryTags)
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
	if metricsPort := os.Getenv(EnvMetricsPort); len(metricsPort) == 0 {
		log.Printf("Environment variable '%s' not specified, defaulting to 8080", EnvMetricsPort)
		cfg.MetricsPort = 8080
	} else {
		port, err := strconv.Atoi(metricsPort)
		if err != nil {
			return fmt.Errorf("invalid value for '%s': %s", EnvMetricsPort, err)
		}
		cfg.MetricsPort = port
	}
	if metrics := strings.ToLower(os.Getenv(EnvMetrics)); len(metrics) != 0 {
		cfg.Metrics = true
	}
	if executionInterval := os.Getenv(EnvExecutionInterval); len(executionInterval) > 0 {
		if interval, err := strconv.Atoi(executionInterval); err != nil {
			return fmt.Errorf("environment variable '%s' must be an integer", EnvExecutionInterval)
		} else {
			cfg.ExecutionInterval = time.Second * time.Duration(interval)
		}
	} else {
		log.Printf("Environment variable '%s' not specified, defaulting to 20 seconds", EnvExecutionInterval)
		cfg.ExecutionInterval = time.Second * 20
	}
	if executionTImeout := os.Getenv(EnvExecutionTimeout); len(executionTImeout) > 0 {
		if timeout, err := strconv.Atoi(executionTImeout); err != nil {
			return fmt.Errorf("environment variable '%s' must be an integer", EnvExecutionTimeout)
		} else {
			cfg.ExecutionTimeout = time.Second * time.Duration(timeout)
		}
	} else {
		log.Printf("Environment variable '%s' not specified, defaulting to 900 seconds", EnvExecutionTimeout)
		cfg.ExecutionTimeout = time.Second * 900
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
		cfg = &config{
			ExecutionInterval: time.Second * 20,
			ExecutionTimeout:  time.Second * 900,
		}
	}
	return cfg
}
