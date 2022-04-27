package topology

import (
	"encoding/json"
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mergeComponentLookup(ctx *SystemContext, name string, spec *v1.CanarySpec) (pkg.Components, error) {
	components := pkg.Components{}
	results, err := lookup(ctx.Kommons, name, *spec)
	if err != nil {
		return nil, errors.Wrapf(err, "component lookup failed: %s", name)
	}
	if len(results) == 1 {
		if err := json.Unmarshal([]byte(results[0].(string)), &components); err != nil {
			return nil, errors.Wrapf(err, "component lookup returned invalid json: %s", name)
		}
	} else {
		// the property returned a list of new properties
		for _, result := range results {
			var p pkg.Component
			data, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &p); err != nil {
				return nil, err
			}
			components = append(components, &p)
		}
	}
	return components, nil
}

func lookupComponents(ctx *SystemContext, component v1.ComponentSpec) ([]*pkg.Component, error) {
	components := pkg.Components{}
	for _, childRaw := range component.Components {
		child := v1.ComponentSpec{}
		if err := json.Unmarshal([]byte(childRaw), &child); err != nil {
			return nil, err
		}
		children, err := lookupComponents(ctx, child)
		if err != nil {
			return nil, err
		}
		components = append(components, children...)
	}

	if len(component.Pods) > 0 {
		lookup := getPodLookup(ctx.Namespace, ctx.SystemAPI.Spec.Pods, component.Pods)
		if children, err := lookupComponents(ctx, lookup); err != nil {
			return nil, err
		} else {
			components = append(components, children...)
		}
	}

	if component.Lookup == nil {
		components = append(components, pkg.NewComponent(component))
	} else {
		logger.Debugf("Looking up components for %s", component.Name)
		if children, err := mergeComponentLookup(ctx, component.Name, component.Lookup); err != nil {
			return nil, err
		} else {
			components = append(components, children...)
		}
	}

	for _, comp := range components {
		for _, property := range component.Properties {
			props, err := lookupProperty(ctx.WithComponents(&components, comp), property)
			if err != nil {
				return nil, errors.Wrapf(err, "property lookup failed: %s", property.Name)
			}
			comp.Properties = append(comp.Properties, props...)
		}

		if comp.Icon == "" && component.Icon != "" {
			comp.Icon = component.Icon
		}
		if comp.Lifecycle == "" && component.Lifecycle != "" {
			comp.Lifecycle = component.Lifecycle
		}
		if comp.ExternalId == "" && component.Id != nil {
			id, err := templating.Template(comp.GetAsEnvironment(), *component.Id)
			if err != nil {
				logger.Errorf("Failed to lookup id: %v", err)
			} else {
				comp.ExternalId = id
			}
		}
		if comp.ExternalId == "" {
			comp.ExternalId = comp.Name
		}
	}

	return components, nil
}

func getPodLookup(namespace string, labels ...map[string]string) v1.ComponentSpec {
	allLabels := map[string]string{}
	for _, label := range labels {
		for k, v := range label {
			allLabels[k] = v
		}
	}

	f := false

	return v1.ComponentSpec{
		Name: "pods",
		Icon: "pod",
		Type: "summary",
		Lookup: &v1.CanarySpec{
			Kubernetes: []v1.KubernetesCheck{
				{
					Kind:  "pod",
					Ready: &f,
					Namespace: v1.ResourceSelector{
						Name: namespace,
					},
					Resource: v1.ResourceSelector{
						LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
							MatchLabels: allLabels,
						}),
					},
					Templatable: v1.Templatable{
						Display: v1.Template{
							Javascript: "JSON.stringify(k8s.getPodTopology(results))",
						},
					},
				},
			},
		},
	}
}

func lookup(client *kommons.Client, name string, spec v1.CanarySpec) ([]interface{}, error) {
	results := []interface{}{}
	for _, result := range checks.RunChecks(context.New(client, v1.NewCanaryFromSpec(name, spec))) {
		if result.Error != "" {
			return nil, fmt.Errorf("%s", result.Error)
		}
		if result.Message != "" {
			results = append(results, result.Message)
		} else if result.Detail == nil {
			return nil, fmt.Errorf("no details returned by lookup, did you specify a display template?")
		} else {
			switch result.Detail.(type) {
			case []interface{}:
				results = append(results, result.Detail.([]interface{})...)
			case interface{}:
				results = append(results, result.Detail)
			default:
				return nil, fmt.Errorf("unknown type %T", result.Detail)
			}
		}
	}
	return results, nil
}

func lookupProperty(ctx *SystemContext, property *v1.Property) (pkg.Properties, error) {
	prop := pkg.NewProperty(*property)
	if property.Lookup == nil {
		return pkg.Properties{prop}, nil
	}

	results, err := lookup(ctx.Kommons, property.Name, *property.Lookup)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		data := []byte(results[0].(string))
		if isComponentList(data) {
			components := pkg.Components{}
			err = json.Unmarshal([]byte(results[0].(string)), &components)
			if err != nil {
				return nil, err
			}
			for _, component := range components {
				found := ctx.Components.Find(component.Name)
				if found == nil {
					return nil, fmt.Errorf("component %s not found", component.Name)
				}
				for _, property := range component.Properties {
					foundProperty := found.Properties.Find(property.Name)
					if foundProperty == nil {
						return nil, fmt.Errorf("property %s not found", property.Name)
					}
					foundProperty.Merge(property)
				}
			}
			return nil, nil
		} else if isPropertyList(data) {
			properties := pkg.Properties{}
			err = json.Unmarshal([]byte(results[0].(string)), &properties)
			return properties, err
		} else {
			logger.Errorf("unknown type %T: %v", data, string(data))
			return nil, nil
		}
	}
	logger.Errorf("unknown type %T", results)

	return nil, nil
}

type TopologyRunOptions struct {
	*kommons.Client
	Depth     int
	Namespace string
}

func Run(opts TopologyRunOptions, s v1.SystemTemplate) []*pkg.System {
	logger.Debugf("Running topology %s depth=%d", s.Name, opts.Depth)
	if s.Namespace == "" {
		s.Namespace = opts.Namespace
	}
	ctx := NewSystemContext(opts.Client, s)
	var results []*pkg.System
	sys := &pkg.System{
		Object: pkg.Object{
			Name:      ctx.SystemAPI.Name,
			Namespace: ctx.SystemAPI.Namespace,
		},
		Tooltip: ctx.SystemAPI.Spec.Tooltip,
		Icon:    ctx.SystemAPI.Spec.Icon,
		Text:    ctx.SystemAPI.Spec.Text,
		Type:    ctx.SystemAPI.Spec.Type,
	}

	if opts.Depth > 0 {
		for _, comp := range ctx.SystemAPI.Spec.Components {
			components, err := lookupComponents(ctx, comp)

			if err != nil {
				logger.Errorf("Error looking up component %s: %s", comp.Name, err)
				continue
			}
			group := pkg.NewComponent(comp)
			for _, component := range components {
				group.Components = append(group.Components, component)
			}
			if comp.Summary == nil {
				group.Summary = group.Components.Summarize()
			} else {
				group.Summary = *comp.Summary
			}
			group.Status = group.Summary.GetStatus()
			sys.Components = append(sys.Components, group)
		}
	}
	ctx.System = sys
	ctx.Components = &sys.Components

	for _, property := range ctx.SystemAPI.Spec.Properties {
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			logger.Errorf("Failed to lookup property %s: %v", property.Name, err)
		} else {
			sys.Properties = append(sys.Properties, props...)
		}
	}
	sys.Summary = sys.Components.Summarize()
	if sys.ID.String() == "" && ctx.SystemAPI.Spec.Id != nil {
		id, err := templating.Template(sys.GetAsEnvironment(), *ctx.SystemAPI.Spec.Id)
		if err != nil {
			logger.Errorf("Failed to lookup id: %v", err)
		} else {
			sys.ID = uuid.MustParse(id)
		}
	}

	if sys.ID.String() == "" {
		sys.ID = uuid.MustParse(sys.Name)
	}
	sys.Status = sys.Summary.GetStatus()
	// if logger.IsTraceEnabled() {
	logger.Debugf(sys.Components.Debug(""))
	// }
	results = append(results, sys)
	logger.Infof("%s id=%s status=%s", sys.Name, sys.ID, sys.Status)
	return results
}
