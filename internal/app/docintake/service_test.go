package docintake_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	appdoc "github.com/dua-os/dua-os-be/internal/app/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fakeDocRepo struct {
	docs []docintake.Document
}

func (f *fakeDocRepo) GetDocumentByHash(_ context.Context, wid id.ID, date, sha string) (docintake.Document, error) {
	for _, d := range f.docs {
		if d.WorkspaceID == wid && d.BusinessDate == date && d.ContentSHA256 == sha {
			return d, nil
		}
	}
	return docintake.Document{}, errx.NotFound("not_found", "not found")
}

func (f *fakeDocRepo) SaveDocument(_ context.Context, d docintake.Document) error {
	f.docs = append(f.docs, d)
	return nil
}

func (f *fakeDocRepo) ListDocuments(_ context.Context, wid id.ID, _ int) ([]docintake.Document, error) {
	var out []docintake.Document
	for _, d := range f.docs {
		if d.WorkspaceID == wid {
			out = append(out, d)
		}
	}
	return out, nil
}

type fakeEnricher struct{ ok bool }

func (e fakeEnricher) EnrichDocument(_ context.Context, _ id.ID, _, _, _ string, base docintake.SimplifiedSummary, _ string) (docintake.SimplifiedSummary, bool) {
	if !e.ok {
		return base, false
	}
	base.Headline = "AI simplified: " + base.Headline
	return base, true
}

func TestIngestDedupAndFallback(t *testing.T) {
	wid := id.New()
	ac := actor.Context{
		Principal: ctxkey.Principal{UserID: id.New().String(), WorkspaceID: wid.String(), Role: identity.RoleManager},
		Workspace: tenancy.Workspace{ID: wid, Kind: tenancy.KindCafe, Name: "Duaa Cafe", Timezone: "UTC"},
		Catalog:   cafe.Catalog{},
		Scope:     staffscope.GeneralManager,
	}
	repo := &fakeDocRepo{}
	svc := appdoc.New(repo, fakeEnricher{ok: false}, clock.Frozen{T: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)})

	first, err := svc.Ingest(t.Context(), ac, appdoc.IngestInput{
		FileName: "daily.csv",
		Content:  "dept,revenue\nBar,1200\nKitchen,800\n",
	})
	if err != nil || first.Status != "fallback_structured" || first.AIUsed || first.Replayed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := svc.Ingest(t.Context(), ac, appdoc.IngestInput{
		FileName: "daily.csv",
		Content:  "dept,revenue\nBar,1200\nKitchen,800\n",
	})
	if err != nil || !second.Replayed || len(repo.docs) != 1 {
		t.Fatalf("dedup failed second=%+v docs=%d", second, len(repo.docs))
	}

	svcAI := appdoc.New(repo, fakeEnricher{ok: true}, clock.Frozen{T: time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)})
	aiDoc, err := svcAI.Ingest(t.Context(), ac, appdoc.IngestInput{
		FileName: "evening.txt",
		Content:  "Arrivals delayed due to airline late flight; 4 rooms out of order.",
	})
	if err != nil || !aiDoc.AIUsed || aiDoc.Status != "analyzed" {
		t.Fatalf("aiDoc=%+v err=%v", aiDoc, err)
	}
}
