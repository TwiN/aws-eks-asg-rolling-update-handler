package cloud_test

import (
	"testing"

	"github.com/TwiN/aws-eks-asg-rolling-update-handler/cloud"
	"github.com/TwiN/aws-eks-asg-rolling-update-handler/cloudtest"
	"github.com/aws/aws-sdk-go/service/autoscaling"
)

func TestDescribeEnabledAutoScalingGroupsByTags(t *testing.T) {
	type testCase struct {
		autoScalingGroups []struct {
			name string
			tags map[string]string
		}
		inputTags   string
		name        string
		outputNames []string
	}

	testCases := []testCase{
		{
			name:        "match foo but not bar",
			inputTags:   "foo=bar",
			outputNames: []string{"foo"},
			autoScalingGroups: []struct {
				name string
				tags map[string]string
			}{
				{
					name: "bar",
					tags: map[string]string{
						"bar": "foo",
					},
				},
				{
					name: "foo",
					tags: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name:        "match foo and bar",
			inputTags:   "foo=bar",
			outputNames: []string{"foo", "bar"},
			autoScalingGroups: []struct {
				name string
				tags map[string]string
			}{
				{
					name: "bar",
					tags: map[string]string{
						"foo": "bar",
					},
				},
				{
					name: "foo",
					tags: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name:        "match foo but not bar with multiple input tags",
			inputTags:   "foo=bar,foobar=true",
			outputNames: []string{"foo"},
			autoScalingGroups: []struct {
				name string
				tags map[string]string
			}{
				{
					name: "bar",
					tags: map[string]string{
						"bar": "foo",
					},
				},
				{
					name: "foo",
					tags: map[string]string{
						"foo":    "bar",
						"foobar": "true",
					},
				},
			},
		},
		{
			name:        "match foo and bar with multiple input tags",
			inputTags:   "foo=bar,foobar=true",
			outputNames: []string{"foo", "bar"},
			autoScalingGroups: []struct {
				name string
				tags map[string]string
			}{
				{
					name: "bar",
					tags: map[string]string{
						"foo":    "bar",
						"foobar": "true",
					},
				},
				{
					name: "foo",
					tags: map[string]string{
						"foo":    "bar",
						"foobar": "true",
					},
				},
			},
		},
	}

	for _, test := range testCases {
		autoScalingGroups := []*autoscaling.Group{}
		for i, asg := range test.autoScalingGroups {
			autoScalingGroup := autoscaling.Group{AutoScalingGroupName: &test.autoScalingGroups[i].name}
			for k, v := range asg.tags {
				key := k
				value := v
				autoScalingGroup.Tags = append(autoScalingGroup.Tags, &autoscaling.TagDescription{
					Key:   &key,
					Value: &value,
				})
			}
			autoScalingGroups = append(autoScalingGroups, &autoScalingGroup)
		}
		svc := cloudtest.NewMockAutoScalingService(autoScalingGroups)
		output, err := cloud.DescribeEnabledAutoScalingGroupsByTags(svc, test.inputTags)
		if err != nil {
			t.Error(err)
		}

		outMap := map[string]bool{}
		for _, outputAutoScalingGroup := range output {
			outMap[*outputAutoScalingGroup.AutoScalingGroupName] = false
		}
		for _, name := range test.outputNames {
			if _, ok := outMap[name]; ok {
				outMap[name] = true
			} else {
				t.Errorf("in '%s', expected '%s' to be present in output: %v", test.name, name, output)
			}
		}
		for name, v := range outMap {
			if !v {
				t.Errorf("in '%s', not expected '%s' to be present in output: %v", test.name, name, output)
			}
		}
	}
}
