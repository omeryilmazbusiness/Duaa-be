package cafeqr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/cafeqr"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fake struct {
	def        cafe.Definition
	codes      map[id.ID]cafe.Code
	tokens     map[string]id.ID
	voices     []cafe.Voice
	recoveries map[id.ID]cafe.Recovery
}

func (f *fake) GetCafeDefinition(_ context.Context, _ id.ID) (cafe.Definition, error) {
	if f.def.WorkspaceID == id.Zero() {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "missing")
	}
	return f.def, nil
}
func (f *fake) SaveCafeDefinition(_ context.Context, def cafe.Definition) error {
	f.def = def
	return nil
}
func (f *fake) GetQRCodeBySite(_ context.Context, _, siteID id.ID) (cafe.Code, error) {
	c, ok := f.codes[siteID]
	if !ok {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "missing")
	}
	return c, nil
}
func (f *fake) GetQRCodeByToken(_ context.Context, token string) (cafe.Code, error) {
	sid, ok := f.tokens[token]
	if !ok {
		return cafe.Code{}, errx.NotFound("qr_code_not_found", "missing")
	}
	return f.codes[sid], nil
}
func (f *fake) SaveQRCode(_ context.Context, c cafe.Code) error {
	if f.codes == nil {
		f.codes = map[id.ID]cafe.Code{}
		f.tokens = map[string]id.ID{}
	}
	if prev, ok := f.codes[c.SiteID]; ok {
		delete(f.tokens, prev.Token)
	}
	f.codes[c.SiteID] = c
	f.tokens[c.Token] = c.SiteID
	return nil
}
func (f *fake) ListQRCodes(_ context.Context, _ id.ID) ([]cafe.Code, error) {
	out := make([]cafe.Code, 0, len(f.codes))
	for _, c := range f.codes {
		out = append(out, c)
	}
	return out, nil
}
func (f *fake) CreateVoice(_ context.Context, v cafe.Voice) error {
	f.voices = append(f.voices, v)
	return nil
}
func (f *fake) GetVoice(_ context.Context, vid id.ID) (cafe.Voice, error) {
	for _, v := range f.voices {
		if v.ID == vid {
			return v, nil
		}
	}
	return cafe.Voice{}, errx.NotFound("voice_not_found", "missing")
}
func (f *fake) ListVoices(_ context.Context, _ id.ID) ([]cafe.Voice, error) { return f.voices, nil }
func (f *fake) CountUnhappyVoicesSince(_ context.Context, _ id.ID, since time.Time) (int, error) {
	n := 0
	for _, v := range f.voices {
		if !v.CreatedAt.Before(since) && v.Unhappy() {
			n++
		}
	}
	return n, nil
}
func (f *fake) GetRecoveryByVoice(_ context.Context, voiceID id.ID) (cafe.Recovery, error) {
	if rec, ok := f.recoveries[voiceID]; ok {
		return rec, nil
	}
	return cafe.Recovery{}, errx.NotFound("recovery_not_found", "missing")
}
func (f *fake) ListRecoveries(_ context.Context, _ id.ID) ([]cafe.Recovery, error) {
	out := make([]cafe.Recovery, 0, len(f.recoveries))
	for _, rec := range f.recoveries {
		out = append(out, rec)
	}
	return out, nil
}
func (f *fake) SaveRecovery(_ context.Context, rec cafe.Recovery) error {
	if f.recoveries == nil {
		f.recoveries = map[id.ID]cafe.Recovery{}
	}
	f.recoveries[rec.VoiceID] = rec
	return nil
}

func TestListMintsTokenWithoutInventingSites(t *testing.T) {
	def := completeDef()
	repo := &fake{def: def}
	svc := cafeqr.New(repo, repo, repo, nil, clock.System{}, "http://localhost:3000")
	list, err := svc.ListCodes(t.Context(), gmActor())
	if err != nil || len(list) != 1 || list[0].Token == "" {
		t.Fatalf("list=%v err=%v", list, err)
	}
}

func TestIssueTableKeepsTokenWhenRenamed(t *testing.T) {
	repo := &fake{}
	svc := cafeqr.New(repo, repo, repo, nil, clock.System{}, "http://localhost:3000")
	first, err := svc.IssueTable(t.Context(), gmActor(), "12")
	if err != nil || first.Token == "" || first.Label != "12" || first.Area != string(cafe.AreaTable) {
		t.Fatalf("issue=%+v err=%v", first, err)
	}
	again, err := svc.IssueTable(t.Context(), gmActor(), "12")
	if err != nil || again.Token != first.Token || again.SiteID != first.SiteID {
		t.Fatalf("reissue must keep the printed code: %+v vs %+v err=%v", first, again, err)
	}
	renamed, err := svc.RenameTable(t.Context(), gmActor(), first.SiteID, "7")
	if err != nil || renamed.Token != first.Token || renamed.Label != "7" {
		t.Fatalf("rename=%+v err=%v", renamed, err)
	}
	if _, err := svc.IssueTable(t.Context(), gmActor(), "7"); err != nil {
		t.Fatal(err)
	}
}

func TestSubmitRequiresReadyWorkshop(t *testing.T) {
	def := completeDef()
	def.SLA.AckMinutes = 0
	repo := &fake{def: def}
	svc := cafeqr.New(repo, repo, repo, nil, clock.System{}, "http://localhost:3000")
	list, _ := svc.ListCodes(t.Context(), gmActor())
	_, err := svc.Submit(t.Context(), list[0].Token, cafeqr.SubmitInput{Rating: 5})
	if err == nil {
		t.Fatal("incomplete workshop must block submit")
	}
}

func gmActor() actor.Context {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	ws := tenancy.Workspace{
		ID: id.MustParse("22222222-2222-4222-8222-222222222222"), Kind: tenancy.KindCafe,
		Name: "Duaa Cafe", Timezone: "Asia/Baghdad", CreatedAt: now, UpdatedAt: now,
	}
	return actor.Context{
		Principal: ctxkey.Principal{UserID: "33333333-3333-4333-8333-333333333333", WorkspaceID: ws.ID.String(), Role: identity.RoleManager},
		Workspace: ws, Catalog: cafe.Catalog{}, Scope: staffscope.GeneralManager,
	}
}

func completeDef() cafe.Definition {
	d := cafe.Seed(id.MustParse("22222222-2222-4222-8222-222222222222"), "Duaa Cafe", "Asia/Baghdad")
	d.Currency = "IQD"
	d.Branches = []cafe.Branch{{Name: "Karada", Active: true}}
	d.QRSites = []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden, Active: true}}
	d.SLA.AckMinutes = 15
	d.Normalize()
	return d
}
