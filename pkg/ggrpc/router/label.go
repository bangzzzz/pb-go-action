package router

import "github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"

// LabelRouter filters instances that satisfy ALL required labels.
type LabelRouter struct {
	required map[string]string
}

// NewLabelRouter returns a Router that keeps only instances matching every
// key-value pair in required.
func NewLabelRouter(required map[string]string) *LabelRouter {
	return &LabelRouter{required: required}
}

// Route returns instances whose labels contain every required label.
func (r *LabelRouter) Route(instances []*registry.Instance) []*registry.Instance {
	if len(r.required) == 0 {
		return instances
	}
	out := make([]*registry.Instance, 0, len(instances))
	for _, inst := range instances {
		if matchLabels(inst.Labels, r.required) {
			out = append(out, inst)
		}
	}
	return out
}

// matchLabels returns true if all entries in required are present in labels.
func matchLabels(labels, required map[string]string) bool {
	for k, v := range required {
		if labels[k] != v {
			return false
		}
	}
	return true
}
