package docintake

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Repository interface {
	GetDocumentByHash(ctx context.Context, workspaceID id.ID, date, sha256 string) (docintake.Document, error)
	SaveDocument(ctx context.Context, doc docintake.Document) error
	ListDocuments(ctx context.Context, workspaceID id.ID, limit int) ([]docintake.Document, error)
}

// AIEnricher is optional: when workspace AI is configured, it generates an executive
// plain-language simplification inside an <untrusted_document> boundary (AI-04).
// On error or nil, deterministic simplification remains intact (AI-05).
type AIEnricher interface {
	EnrichDocument(ctx context.Context, workspaceID id.ID, locale, fileName, sourceKind string, base docintake.SimplifiedSummary, excerpt string) (docintake.SimplifiedSummary, bool)
}
