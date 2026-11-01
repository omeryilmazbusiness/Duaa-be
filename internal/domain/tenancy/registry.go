package tenancy

import "github.com/dua-os/dua-os-be/internal/shared/errx"

// Registry maps workspace kinds to vertical catalogs.
// Composition root registers cafe first; hotel/supply catalogs can be added without
// changing identity, RBAC or task modules (Open/Closed).
type Registry struct {
	byKind map[Kind]Catalog
}

func NewRegistry(catalogs ...Catalog) *Registry {
	r := &Registry{byKind: make(map[Kind]Catalog, len(catalogs))}
	for _, c := range catalogs {
		r.byKind[c.Kind()] = c
	}
	return r
}

func (r *Registry) Catalog(kind Kind) (Catalog, error) {
	c, ok := r.byKind[kind]
	if !ok {
		return nil, errx.Invalid("unsupported_workspace_kind", "this workspace kind is not registered")
	}
	return c, nil
}

func (r *Registry) Must(kind Kind) Catalog {
	c, err := r.Catalog(kind)
	if err != nil {
		panic(err)
	}
	return c
}

func (r *Registry) Registered() []Kind {
	out := make([]Kind, 0, len(r.byKind))
	for k := range r.byKind {
		out = append(out, k)
	}
	return out
}
