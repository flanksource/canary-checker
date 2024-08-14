package runner

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsCanaryIgnored(t *testing.T) {
	tests := []struct {
		name             string
		canary           *metav1.ObjectMeta
		want             bool
		IncludeNamespace []string
		IncludeLabels    []string
	}{
		{
			name: "namespace included",
			canary: &metav1.ObjectMeta{
				Namespace: "default",
			},
			want:             false,
			IncludeNamespace: []string{"default"},
		},
		{
			name: "namespace excluded",
			canary: &metav1.ObjectMeta{
				Namespace: "canaries",
			},
			want:             true,
			IncludeNamespace: []string{"default"},
		},
		{
			name: "label included",
			canary: &metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "prod",
				},
			},
			want:          false,
			IncludeLabels: []string{"env=prod"},
		},
		{
			name: "label excluded",
			canary: &metav1.ObjectMeta{
				Labels: map[string]string{
					"env": "dev",
				},
			},
			want:          true,
			IncludeLabels: []string{"env=prod"},
		},
		{
			name: "multiple labels included",
			canary: &metav1.ObjectMeta{
				Labels: map[string]string{
					"env":    "prod",
					"region": "us-east-1",
				},
			},
			want:          false,
			IncludeLabels: []string{"env=prod", "region=us-east-1"},
		},
		{
			name: "multiple labels excluded",
			canary: &metav1.ObjectMeta{
				Labels: map[string]string{
					"env":    "prod",
					"region": "eu-west-2",
				},
			},
			want:          true,
			IncludeLabels: []string{"env=prod", "region=us-east-1"},
		},
		{
			name: "labels with matchItems",
			canary: &metav1.ObjectMeta{
				Labels: map[string]string{
					"env":    "prod",
					"region": "us-east-2",
				},
			},
			want:          false,
			IncludeLabels: []string{"env=prod", "region=us-*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			IncludeNamespaces = tt.IncludeNamespace
			IncludeLabels = tt.IncludeLabels

			if got := IsCanaryIgnored(tt.canary); got != tt.want {
				t.Errorf("IsCanaryIgnored() = %v, want %v", got, tt.want)
			}
		})
	}
}
