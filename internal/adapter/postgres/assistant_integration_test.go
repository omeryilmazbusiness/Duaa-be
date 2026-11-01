package postgres_test

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pgadapter "github.com/dua-os/dua-os-be/internal/adapter/postgres"
	"github.com/dua-os/dua-os-be/internal/bootstrap"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/platform/postgres"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

// Runs against a migrated database: TEST_DATABASE_URL=postgres://... go test ./internal/adapter/postgres
func TestAssistantStoreAgainstPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := t.Context()
	pool, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := pgadapter.New(pool)
	seed, err := bootstrap.SeedCafe(ctx, store, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.GetAssistantSettings(ctx, seed.CafeID); !isNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
	gm, err := store.GetUserByEmail(ctx, seed.GMEmail)
	if err != nil {
		t.Fatal(err)
	}
	user := gm.ID
	now := time.Now().UTC().Truncate(time.Microsecond)
	s := assistant.Settings{
		WorkspaceID: seed.CafeID, Provider: assistant.ProviderGemini, Model: "gemini-2.0-flash",
		EncryptedKey: "v1.xxx", KeyHint: "abcd", DailyLimit: 5, Enabled: true,
		UpdatedByUserID: user, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.SaveAssistantSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	s.Model, s.DailyLimit = "gemini-2.5-flash", 7
	if err := store.SaveAssistantSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAssistantSettings(ctx, seed.CafeID)
	if err != nil || got.Provider != assistant.ProviderGemini || got.Model != "gemini-2.5-flash" || got.DailyLimit != 7 {
		t.Fatalf("upsert round trip: %+v %v", got, err)
	}

	// 20 concurrent reservations against a limit of 5 must grant exactly 5.
	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := store.ReserveAIUsage(ctx, seed.CafeID, "2026-11-01", 5); err == nil && ok {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 5 {
		t.Fatalf("granted %d reservations, want exactly 5", granted.Load())
	}
	if err := store.ReleaseAIUsage(ctx, seed.CafeID, "2026-11-01"); err != nil {
		t.Fatal(err)
	}
	if used, _ := store.AIUsage(ctx, seed.CafeID, "2026-11-01"); used != 4 {
		t.Fatalf("used = %d after release, want 4", used)
	}
	if err := store.DeleteAssistantSettings(ctx, seed.CafeID); err != nil {
		t.Fatal(err)
	}
}

func isNotFound(err error) bool { return errors.Is(err, errx.ErrNotFound) }
