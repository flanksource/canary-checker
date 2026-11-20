package checks

import (
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"github.com/samber/lo"
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
				"key1": {Value: lo.ToPtr("value1")},
				"key2": {Value: lo.ToPtr("value2")},
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
				"key1": {Value: lo.ToPtr("value1")},
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
				"key1": {Value: lo.ToPtr("value1")},
				"key2": {Value: lo.ToPtr("value3")},
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

func TestGetRunArgsSkipsInProgressRun(t *testing.T) {
	runs := []pipelines.Run{
		{Id: lo.ToPtr(3), State: &pipelines.RunStateValues.InProgress},
		{Id: lo.ToPtr(2), State: &pipelines.RunStateValues.Completed},
		{Id: lo.ToPtr(1), State: &pipelines.RunStateValues.Completed},
	}

	args := getRunArgs("proj", lo.ToPtr(7), getLatestCompletedRun(runs))
	if args.RunId == nil || *args.RunId != 2 {
		t.Fatalf("expected the latest completed run (id 2), got %v", args.RunId)
	}
}
