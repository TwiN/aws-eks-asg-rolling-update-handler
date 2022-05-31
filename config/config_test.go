package config

import (
	"os"
	"reflect"
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

func TestInitialize_withClusterName(t *testing.T) {
	_ = os.Setenv(EnvClusterName, "foo")
	_ = os.Setenv(EnvAutodiscoveryTags, "foo=bar")
	_ = os.Setenv(EnvAutoScalingGroupNames, "foo,bar")
	defer os.Clearenv()
	_ = Initialize()
	config := Get()
	if config.AutodiscoveryTags != "k8s.io/cluster-autoscaler/foo=owned,k8s.io/cluster-autoscaler/enabled=true" {
		t.Error()
	} else if len(config.AutoScalingGroupNames) != 0 {
		t.Error()
	}
}

func TestInitialize_withAutodiscoveryTags(t *testing.T) {
	_ = os.Unsetenv(EnvClusterName)
	_ = os.Setenv(EnvAutodiscoveryTags, "foo=bar,foobar=true")
	_ = os.Setenv(EnvAutoScalingGroupNames, "foo,bar")
	defer os.Clearenv()
	_ = Initialize()
	config := Get()
	if config.AutodiscoveryTags != "foo=bar,foobar=true" {
		t.Error()
	} else if len(config.AutoScalingGroupNames) != 0 {
		t.Error()
	}
}

func TestInitialize_withAutoScalingGroupNames(t *testing.T) {
	_ = os.Unsetenv(EnvClusterName)
	_ = os.Unsetenv(EnvAutodiscoveryTags)
	_ = os.Setenv(EnvAutoScalingGroupNames, "foo,bar")
	defer os.Clearenv()
	_ = Initialize()
	config := Get()
	if !reflect.DeepEqual(config.AutoScalingGroupNames, []string{"foo", "bar"}) {
		t.Error()
	}
}
