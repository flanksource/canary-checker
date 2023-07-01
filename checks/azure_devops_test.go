package checks

import (
	"github.com/flanksource/canary-checker/pkg/utils"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
)

func TestMatchPipelineVariables(t *testing.T) {
	tests := []struct {
		name       string
		want       map[string]string
		got        *map[string]pipelines.Variable
		wantResult bool
	}{
		{
			name:       "Empty want and got",
			want:       map[string]string{},
			got:        &map[string]pipelines.Variable{},
			wantResult: true,
		},
		{
			name: "Equal want and got",
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			got: &map[string]pipelines.Variable{
				"key1": {Value: utils.Ptr("value1")},
				"key2": {Value: utils.Ptr("value2")},
			},
			wantResult: true,
		},
		{
			name: "Missing key in got",
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			got: &map[string]pipelines.Variable{
				"key1": {Value: utils.Ptr("value1")},
			},
			wantResult: false,
		},
		{
			name: "Different value in got",
			want: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			got: &map[string]pipelines.Variable{
				"key1": {Value: utils.Ptr("value1")},
				"key2": {Value: utils.Ptr("value3")},
			},
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResult := matchPipelineVariables(tt.want, tt.got); gotResult != tt.wantResult {
				t.Errorf("matchPipelineVariables() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
