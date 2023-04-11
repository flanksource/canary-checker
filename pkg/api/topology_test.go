package api

import (
	"testing"

	"github.com/flanksource/duty/models"
	"github.com/google/go-cmp/cmp"
)

func Test_populateTopologyResult(t *testing.T) {
	type args struct {
		components models.Components
		res        TopologyRes
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "first",
			args: args{
				res: TopologyRes{
					Types:          []string{"type-1", "type-1-1", "type-1-2", "type-2", "type-2-1", "type-2-1-1", "type-2-1-2", "type-2-1-3", "type-2-2"},
					HealthStatuses: []string{"OK"},
					Tags: []Tag{
						{Key: "tag", Val: "tag-1"},
						{Key: "tag", Val: "tag-1-1"},
						{Key: "tag", Val: "tag-1-2"},
						{Key: "tag", Val: "tag-2"},
						{Key: "tag", Val: "tag-2-1"},
						{Key: "tag", Val: "tag-2-1-1"},
						{Key: "tag", Val: "tag-2-1-2"},
						{Key: "tag", Val: "tag-2-1-3"},
						{Key: "tag", Val: "tag-2-2"},
					},
				},
				components: models.Components{
					{
						Name:   "first",
						Status: "OK",
						Type:   "type-1",
						Labels: map[string]string{
							"tag": "tag-1",
						},
						Components: models.Components{
							{
								Name:   "first-first",
								Status: "OK",
								Type:   "type-1-1",
								Labels: map[string]string{
									"tag": "tag-1-1",
								},
							},
							{
								Name:   "first-second",
								Status: "OK",
								Type:   "type-1-2",
								Labels: map[string]string{
									"tag": "tag-1-2",
								},
							},
						},
					},
					{
						Name:   "second",
						Status: "OK",
						Type:   "type-2",
						Labels: map[string]string{
							"tag": "tag-2",
						},
						Components: models.Components{
							{
								Name:   "second-first",
								Status: "OK",
								Type:   "type-2-1",
								Labels: map[string]string{
									"tag": "tag-2-1",
								},
								Components: models.Components{
									{
										Name:   "second-first-first",
										Status: "OK",
										Type:   "type-2-1-1",
										Labels: map[string]string{
											"tag": "tag-2-1-1",
										},
									},
									{
										Name:   "second-first-second",
										Status: "OK",
										Type:   "type-2-1-2",
										Labels: map[string]string{
											"tag": "tag-2-1-2",
										},
									},
									{
										Name:   "second-first-third",
										Status: "OK",
										Type:   "type-2-1-3",
										Labels: map[string]string{
											"tag": "tag-2-1-3",
										},
									},
								},
							},
							{
								Name:   "second-second",
								Status: "OK",
								Type:   "type-2-2",
								Labels: map[string]string{
									"tag": "tag-2-2",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res TopologyRes
			populateTopologyResult(tt.args.components, &res)
			if diff := cmp.Diff(tt.args.res, res); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
