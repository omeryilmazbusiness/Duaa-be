package docintake_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestNormalizeAndSimplifyCSVComputesCoverageAndMissingCells(t *testing.T) {
	csv := "item,qty,revenue\nEspresso,100,450\nLatte,,300\n"
	doc, err := docintake.NormalizeDocument(id.New(), id.New(), "2026-11-18", "sales.csv", "", csv, time.Now())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if doc.SourceKind != docintake.KindCSV {
		t.Fatalf("kind=%s", doc.SourceKind)
	}
	if doc.Summary.RowCount != 2 || doc.Summary.MissingCells != 1 || doc.DataCoveragePct != 83 {
		t.Fatalf("summary=%+v", doc.Summary)
	}
	if len(doc.Summary.Risks) == 0 {
		t.Fatal("expected missing cell risk warning")
	}
}

func TestNormalizeRejectsEmptyAndOversize(t *testing.T) {
	if _, err := docintake.NormalizeDocument(id.New(), id.New(), "2026-11-18", "a.txt", "", "   ", time.Now()); err == nil {
		t.Fatal("empty content must fail")
	}
	huge := make([]byte, docintake.MaxFileBytes+10)
	for i := range huge {
		huge[i] = 'a'
	}
	if _, err := docintake.NormalizeDocument(id.New(), id.New(), "2026-11-18", "big.txt", "", string(huge), time.Now()); err == nil {
		t.Fatal("oversize content must fail")
	}
}
