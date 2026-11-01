package memory_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/adapter/memory"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestAssistantSettingsSurviveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "assistant-settings.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	a := memory.New()
	if err := a.AttachAIPersist(path); err != nil {
		t.Fatal(err)
	}
	err := a.SaveAssistantSettings(t.Context(), assistant.Settings{
		WorkspaceID: wid, Provider: assistant.ProviderGemini, Model: "gemini-2.0-flash",
		EncryptedKey: "sealed-not-plaintext", KeyHint: "WXYZ", DailyLimit: 30, Enabled: true,
		UpdatedByUserID: id.MustParse("33333333-3333-4333-8333-333333333333"),
		CreatedAt:       now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	b := memory.New()
	if err := b.AttachAIPersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetAssistantSettings(t.Context(), wid)
	if err != nil || got.EncryptedKey != "sealed-not-plaintext" || got.Model != "gemini-2.0-flash" || !got.Enabled {
		t.Fatalf("reloaded = %+v, err = %v", got, err)
	}
}

func TestCafeDefinitionSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-definition.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	a := memory.New()
	if err := a.AttachCafePersist(path); err != nil {
		t.Fatal(err)
	}
	def := cafe.Seed(wid, "Duaa Cafe", "Asia/Baghdad")
	def.Currency = "IQD"
	def.Branches = []cafe.Branch{{Name: "Karada", Active: true}}
	def.QRSites = []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden, Active: true}}
	def.SLA.AckMinutes = 15
	def.UpdatedAt = now
	def.Normalize()
	if err := a.SaveCafeDefinition(t.Context(), def); err != nil {
		t.Fatal(err)
	}

	b := memory.New()
	if err := b.AttachCafePersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetCafeDefinition(t.Context(), wid)
	if err != nil || got.DisplayName != "Duaa Cafe" || got.Currency != "IQD" || !got.Completeness().HasBranch {
		t.Fatalf("reloaded = %+v, err = %v", got, err)
	}
}

func TestQRCodeAndVoiceSurviveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-qr.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	sid := id.MustParse("55555555-5555-4555-8555-555555555555")
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tok, err := cafe.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	a := memory.New()
	if err := a.AttachQRPersist(path); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveQRCode(t.Context(), cafe.Code{WorkspaceID: wid, SiteID: sid, Token: tok, IssuedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := a.CreateVoice(t.Context(), cafe.Voice{
		ID: id.New(), WorkspaceID: wid, SiteID: sid, Token: tok, Rating: 2, Comment: "cold", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	b := memory.New()
	if err := b.AttachQRPersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetQRCodeByToken(t.Context(), tok)
	if err != nil || got.SiteID != sid {
		t.Fatalf("code = %+v err=%v", got, err)
	}
	voices, err := b.ListVoices(t.Context(), wid)
	if err != nil || len(voices) != 1 || voices[0].Comment != "cold" {
		t.Fatalf("voices=%v err=%v", voices, err)
	}
}

func TestIntakeSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-intake.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	a := memory.New()
	if err := a.AttachIntakePersist(path); err != nil {
		t.Fatal(err)
	}
	rec, err := cafe.UpsertIntake(nil, wid, id.New(), "2026-11-01", "", "quiet close", "", "", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveIntake(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	b := memory.New()
	if err := b.AttachIntakePersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetIntake(t.Context(), wid, "2026-11-01")
	if err != nil || got.CloseNote != "quiet close" {
		t.Fatalf("reload=%+v err=%v", got, err)
	}
}

func TestHandoverSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-handover.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 2, 18, 0, 0, 0, time.UTC)
	a := memory.New()
	if err := a.AttachHandoverPersist(path); err != nil {
		t.Fatal(err)
	}
	rec, err := cafe.OpenHandover(wid, id.New(), "2026-11-02", "", "", "keys on bar", nil, nil, nil, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveHandover(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	b := memory.New()
	if err := b.AttachHandoverPersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetOpenHandover(t.Context(), wid)
	if err != nil || got.Note != "keys on bar" || !got.Open() {
		t.Fatalf("reload=%+v err=%v", got, err)
	}
}

func TestChecklistSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-checklist.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	a := memory.New()
	if err := a.AttachCheckPersist(path); err != nil {
		t.Fatal(err)
	}
	tpl, err := cafe.ReplaceTemplate(wid, id.New(), []cafe.CheckItem{
		{Kind: cafe.CheckOpen, Title: "chairs down", Active: true},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveChecklistTemplate(t.Context(), tpl); err != nil {
		t.Fatal(err)
	}
	run, err := cafe.ToggleRun(nil, wid, id.New(), "2026-11-02", tpl.Items[0], now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveChecklistRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	b := memory.New()
	if err := b.AttachCheckPersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetChecklistTemplate(t.Context(), wid)
	if err != nil || len(got.Items) != 1 || got.Items[0].Title != "chairs down" {
		t.Fatalf("template=%+v err=%v", got, err)
	}
	live, err := b.GetChecklistRun(t.Context(), wid, "2026-11-02", cafe.CheckOpen)
	if err != nil || !live.Marked(got.Items[0].ID) {
		t.Fatalf("run=%+v err=%v", live, err)
	}
}

func TestPOSTicketsSurviveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-pos.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	a := memory.New()
	if err := a.AttachPOSPersist(path); err != nil {
		t.Fatal(err)
	}
	ticket, err := cafe.NewTicket(wid, id.New(), "2026-11-02", "S-1", "sale", "cash", 2, 8000, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CreatePOSTicket(t.Context(), ticket); err != nil {
		t.Fatal(err)
	}
	b := memory.New()
	if err := b.AttachPOSPersist(path); err != nil {
		t.Fatal(err)
	}
	list, err := b.ListPOSTickets(t.Context(), wid, "2026-11-02")
	if err != nil || len(list) != 1 || list[0].Net != 8000 {
		t.Fatalf("reload=%+v err=%v", list, err)
	}
}

func TestApprovalsSurviveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spend-approvals.json")
	wid := id.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	a := memory.New()
	if err := a.AttachApprovalPersist(path); err != nil {
		t.Fatal(err)
	}
	pack, err := finance.NewApproval(wid, id.New(), "Milk invoice", "Al-Nahrain", "IQD", "photo", 250000, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CreateApproval(t.Context(), pack); err != nil {
		t.Fatal(err)
	}
	b := memory.New()
	if err := b.AttachApprovalPersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetApproval(t.Context(), pack.ID)
	if err != nil || got.Title != "Milk invoice" || got.Version != 1 || got.Evidence != "photo" {
		t.Fatalf("reload=%+v err=%v", got, err)
	}
}
