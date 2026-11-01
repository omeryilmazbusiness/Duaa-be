package memory_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/adapter/memory"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
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
