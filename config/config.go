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
	EnvEnvironment               = "ENVIRONMENT"
	EnvDebug                     = "DEBUG"
	EnvIgnoreDaemonSets          = "IGNORE_DAEMON_SETS"
	EnvDeleteLocalData           = "DELETE_LOCAL_DATA" // Deprecated: in favor of DeleteEmptyDirData (DELETE_EMPTY_DIR_DATA)
	EnvDeleteEmptyDirData        = "DELETE_EMPTY_DIR_DATA"
	EnvClusterName               = "CLUSTER_NAME"
	EnvAutodiscoveryTags         = "AUTODISCOVERY_TAGS"
	EnvAutoScalingGroupNames     = "AUTO_SCALING_GROUP_NAMES"
	EnvAwsRegion                 = "AWS_REGION"
	EnvExecutionInterval         = "EXECUTION_INTERVAL"
	EnvExecutionTimeout          = "EXECUTION_TIMEOUT"
	EnvPodTerminationGracePeriod = "POD_TERMINATION_GRACE_PERIOD"
	EnvMetrics                   = "METRICS"
	EnvMetricsPort               = "METRICS_PORT"
	EnvSlowMode                  = "SLOW_MODE"
	EnvEagerCordoning            = "EAGER_CORDONING"
)

type config struct {
	Environment               string        // Optional
	Debug                     bool          // Defaults to false
	AutoScalingGroupNames     []string      // Required if AutodiscoveryTags not provided
	AutodiscoveryTags         string        // Required if AutoScalingGroupNames not provided
	AwsRegion                 string        // Defaults to us-west-2
	IgnoreDaemonSets          bool          // Defaults to true
	DeleteEmptyDirData        bool          // Defaults to true
	ExecutionInterval         time.Duration // Defaults to 20s
	ExecutionTimeout          time.Duration // Defaults to 900s
	PodTerminationGracePeriod int           // Defaults to -1
	Metrics                   bool          // Defaults to false
	MetricsPort               int           // Defaults to 8080
	SlowMode                  bool          // Defaults to false
	EagerCordoning            bool          // Defaults to false
}

// Initialize is used to initialize the application's configuration
func Initialize() error {
	cfg = &config{
		Environment:    strings.ToLower(os.Getenv(EnvEnvironment)),
		Debug:          strings.ToLower(os.Getenv(EnvDebug)) == "true",
		SlowMode:       strings.ToLower(os.Getenv(EnvSlowMode)) == "true",
		EagerCordoning: strings.ToLower(os.Getenv(EnvEagerCordoning)) == "true",
	}
	if clusterName := os.Getenv(EnvClusterName); len(clusterName) > 0 {
		// See "Prerequisites" in https://docs.aws.amazon.com/eks/latest/userguide/autoscaling.html
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
	// if the deprecated EnvDeleteLocalData is set, we need to set EnvDeleteEmptyDirData to its value
	if deleteLocalData := strings.ToLower(os.Getenv(EnvDeleteLocalData)); len(deleteLocalData) > 0 {
		log.Println("NOTICE: Environment variable '" + EnvDeleteLocalData + "' has been deprecated in favor of '" + EnvDeleteEmptyDirData + "'.")
		log.Println("NOTICE: Make sure to update your configuration, as said deprecated environment variable will be removed in a future release.")
		if len(os.Getenv(EnvDeleteEmptyDirData)) == 0 {
			_ = os.Setenv(EnvDeleteEmptyDirData, deleteLocalData)
		} else {
			log.Println("WARNING: Both '" + EnvDeleteLocalData + "' and '" + EnvDeleteEmptyDirData + "' are set. The former is deprecated, and will be ignored.")
		}
	}
	if deleteEmptyDirData := strings.ToLower(os.Getenv(EnvDeleteEmptyDirData)); len(deleteEmptyDirData) == 0 || deleteEmptyDirData == "true" {
		cfg.DeleteEmptyDirData = true
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
	if executionTimeout := os.Getenv(EnvExecutionTimeout); len(executionTimeout) > 0 {
		if timeout, err := strconv.Atoi(executionTimeout); err != nil {
			return fmt.Errorf("environment variable '%s' must be an integer", EnvExecutionTimeout)
		} else {
			cfg.ExecutionTimeout = time.Second * time.Duration(timeout)
		}
	} else {
		log.Printf("Environment variable '%s' not specified, defaulting to 900 seconds", EnvExecutionTimeout)
		cfg.ExecutionTimeout = time.Second * 900
	}
	if terminationGracePeriod := os.Getenv(EnvPodTerminationGracePeriod); len(terminationGracePeriod) > 0 {
		if gracePeriod, err := strconv.Atoi(terminationGracePeriod); err != nil {
			return fmt.Errorf("environment variable '%s' must be an integer", EnvPodTerminationGracePeriod)
		} else {
			cfg.PodTerminationGracePeriod = gracePeriod
		}
	} else {
		log.Printf("Environment variable '%s' not specified, defaulting to -1 (pod's terminationGracePeriodSeconds)", EnvPodTerminationGracePeriod)
		cfg.PodTerminationGracePeriod = -1
	}
	return nil
}

// Set sets the application's configuration and is intended to be used for testing purposes.
// See Initialize() for production
func Set(autoScalingGroupNames []string, ignoreDaemonSets, deleteEmptyDirData bool) {
	cfg = &config{
		AutoScalingGroupNames: autoScalingGroupNames,
		IgnoreDaemonSets:      ignoreDaemonSets,
		DeleteEmptyDirData:    deleteEmptyDirData,
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
