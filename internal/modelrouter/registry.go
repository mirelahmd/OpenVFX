package modelrouter

type Registry struct {
	adapters []Adapter
}

func NewRegistry() *Registry {
	r := &Registry{}
	r.Register(NewDryRunAdapter())
	r.Register(NewStubAdapter())
	r.Register(NewOllamaAdapter())
	return r
}

func (r *Registry) Register(adapter Adapter) {
	if adapter == nil {
		return
	}
	r.adapters = append(r.adapters, adapter)
}

func (r *Registry) ByName(name string) (Adapter, bool) {
	for _, adapter := range r.adapters {
		if adapter.Name() == name {
			return adapter, true
		}
	}
	return nil, false
}

func (r *Registry) ForProvider(provider string) (Adapter, bool) {
	for _, adapter := range r.adapters {
		if adapter.Supports(provider) {
			return adapter, true
		}
	}
	return nil, false
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		out = append(out, adapter.Name())
	}
	return out
}

var defaultRegistry = NewRegistry()

func DefaultRegistry() *Registry {
	return defaultRegistry
}
