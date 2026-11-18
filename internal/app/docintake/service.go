package docintake

import (
	"context"
	"errors"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	repo     Repository
	enricher AIEnricher
	clock    clock.Clock
}

func New(repo Repository, enricher AIEnricher, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{repo: repo, enricher: enricher, clock: clk}
}

type DocumentView struct {
	ID              string                    `json:"id"`
	WorkspaceID     string                    `json:"workspaceId"`
	BusinessDate    string                    `json:"businessDate"`
	FileName        string                    `json:"fileName"`
	SourceKind      string                    `json:"sourceKind"`
	Status          string                    `json:"status"`
	AIUsed          bool                      `json:"aiUsed"`
	DataCoveragePct int                       `json:"dataCoveragePct"`
	Summary         docintake.SimplifiedSummary `json:"summary"`
	Replayed        bool                      `json:"replayed,omitempty"`
	CreatedAt       string                    `json:"createdAt"`
}

type IngestInput struct {
	BusinessDate string
	FileName     string
	SourceKind   string
	Content      string
	Locale       string
}

func (s *Service) Ingest(ctx context.Context, ac actor.Context, in IngestInput) (DocumentView, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) && !rbac.Allows(ac.Scope, rbac.SubmitReport) {
		return DocumentView{}, errx.Forbidden("forbidden", "cannot upload documents")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return DocumentView{}, errx.Unauthenticated("invalid token")
	}
	now := s.clock.Now().UTC()
	day := in.BusinessDate
	if day == "" {
		day = finance.BusinessDate(now, ac.Workspace.Timezone)
	}
	doc, err := docintake.NormalizeDocument(ac.Workspace.ID, uid, day, in.FileName, in.SourceKind, in.Content, now)
	if err != nil {
		return DocumentView{}, err
	}
	existing, err := s.repo.GetDocumentByHash(ctx, ac.Workspace.ID, doc.BusinessDate, doc.ContentSHA256)
	if err == nil {
		v := toView(existing)
		v.Replayed = true
		return v, nil
	}
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return DocumentView{}, err
	}

	if s.enricher != nil {
		if enriched, ok := s.enricher.EnrichDocument(ctx, ac.Workspace.ID, in.Locale, doc.FileName, string(doc.SourceKind), doc.Summary, doc.RawExcerpt); ok {
			doc.Summary = enriched
			doc.Status = docintake.StatusAnalyzed
			doc.AIUsed = true
		}
	}
	if err := s.repo.SaveDocument(ctx, doc); err != nil {
		return DocumentView{}, err
	}
	return toView(doc), nil
}

func (s *Service) List(ctx context.Context, ac actor.Context) ([]DocumentView, error) {
	if !rbac.Allows(ac.Scope, rbac.ViewDashboard) {
		return nil, errx.Forbidden("forbidden", "cannot view documents")
	}
	rows, err := s.repo.ListDocuments(ctx, ac.Workspace.ID, 30)
	if err != nil {
		return nil, err
	}
	out := make([]DocumentView, 0, len(rows))
	for _, r := range rows {
		out = append(out, toView(r))
	}
	return out, nil
}

func toView(d docintake.Document) DocumentView {
	if d.Summary.KeyMetrics == nil {
		d.Summary.KeyMetrics = []docintake.KeyMetric{}
	}
	if d.Summary.Takeaways == nil {
		d.Summary.Takeaways = []string{}
	}
	if d.Summary.Risks == nil {
		d.Summary.Risks = []string{}
	}
	if d.Summary.Actions == nil {
		d.Summary.Actions = []string{}
	}
	return DocumentView{
		ID:              d.ID.String(),
		WorkspaceID:     d.WorkspaceID.String(),
		BusinessDate:    d.BusinessDate,
		FileName:        d.FileName,
		SourceKind:      string(d.SourceKind),
		Status:          string(d.Status),
		AIUsed:          d.AIUsed,
		DataCoveragePct: d.DataCoveragePct,
		Summary:         d.Summary,
		CreatedAt:       d.CreatedAt.UTC().Format(time.RFC3339),
	}
}
