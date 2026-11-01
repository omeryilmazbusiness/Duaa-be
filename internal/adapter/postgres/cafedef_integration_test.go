package postgres_test

import (
	"os"
	"testing"
	"time"

	pgadapter "github.com/dua-os/dua-os-be/internal/adapter/postgres"
	"github.com/dua-os/dua-os-be/internal/bootstrap"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/platform/postgres"
)

func TestCafeDefinitionAgainstPostgres(t *testing.T) {
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
	if _, err := store.GetCafeDefinition(ctx, seed.CafeID); !isNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
	def := cafe.Seed(seed.CafeID, "Duaa Cafe", "Asia/Baghdad")
	def.Currency = "IQD"
	def.Branches = []cafe.Branch{{Name: "Karada", Active: true}}
	def.QRSites = []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden, Active: true}}
	def.SLA.AckMinutes = 15
	def.UpdatedAt = time.Now().UTC()
	def.Normalize()
	if err := store.SaveCafeDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetCafeDefinition(ctx, seed.CafeID)
	if err != nil || got.DisplayName != "Duaa Cafe" || got.Currency != "IQD" || !got.Completeness().HasBranch {
		t.Fatalf("round trip = %+v err=%v", got, err)
	}
	def.DisplayName = "Duaa Cafe Karada"
	if err := store.SaveCafeDefinition(ctx, def); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetCafeDefinition(ctx, seed.CafeID)
	if err != nil || got.DisplayName != "Duaa Cafe Karada" {
		t.Fatalf("upsert = %+v err=%v", got, err)
	}
}
