package tenancy

import "github.com/dua-os/dua-os-be/internal/shared/errx"

// Kind is a vertical (industry) plugged into the kernel.
// New companies (hotel, supply) register a catalog — they do not fork identity/RBAC/tasks.
type Kind string

const (
	KindCafe   Kind = "cafe"
	KindHotel  Kind = "hotel"
	KindSupply Kind = "supply"
)

func ParseKind(raw string) (Kind, error) {
	switch Kind(raw) {
	case KindCafe, KindHotel, KindSupply:
		return Kind(raw), nil
	default:
		return "", errx.Invalid("invalid_workspace_kind", "workspace kind must be cafe, hotel or supply")
	}
}

func (k Kind) Valid() bool {
	return k == KindCafe || k == KindHotel || k == KindSupply
}
