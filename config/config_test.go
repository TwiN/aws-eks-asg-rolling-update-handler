package config

import (
	"os"
	"testing"
)

func TestInitialize(t *testing.T) {
	_ = os.Setenv(EnvAutoScalingGroupNames, "asg-a,asg-b,asg-c")
	_ = os.Setenv(EnvIgnoreDaemonSets, "false")
	_ = os.Setenv(EnvDeleteLocalData, "false")
	defer os.Clearenv()
	_ = Initialize()
	config := Get()
	if len(config.AutoScalingGroupNames) != 3 {
		t.Error()
	}
	if config.IgnoreDaemonSets {
		t.Error()
	}
	if config.DeleteLocalData {
		t.Error()
	}
}

func TestInitialize_withDefaultNonRequiredValues(t *testing.T) {
	_ = os.Setenv(EnvAutoScalingGroupNames, "asg-a,asg-b,asg-c")
	defer os.Clearenv()
	_ = Initialize()
	config := Get()
	if len(config.AutoScalingGroupNames) != 3 {
		t.Error()
	}
	if !config.IgnoreDaemonSets {
		t.Error("should've defaulted to ignoring daemon sets")
	}
	if !config.DeleteLocalData {
		t.Error("should've defaulted to deleting local data")
	}
}

func TestInitialize_withMissingRequiredValues(t *testing.T) {
	if err := Initialize(); err == nil {
		t.Error("expected error because required environment variables are missing")
	}
}

func TestSet(t *testing.T) {
	Set([]string{"asg-a", "asg-b", "asg-c"}, true, true)
	config := Get()
	if len(config.AutoScalingGroupNames) != 3 {
		t.Error()
	}
	if !config.IgnoreDaemonSets {
		t.Error()
	}
	if !config.DeleteLocalData {
		t.Error()
	}
}
