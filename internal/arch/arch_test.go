package arch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundaries(t *testing.T) {
	assertNoImport(t, "internal/domain", "github.com/dua-os/dua-os-be/internal/adapter")
	assertNoImport(t, "internal/domain", "github.com/dua-os/dua-os-be/internal/app")
	assertNoImport(t, "internal/domain", "github.com/dua-os/dua-os-be/internal/platform")
	assertNoImport(t, "internal/app", "github.com/dua-os/dua-os-be/internal/adapter")
	assertNoImport(t, "internal/domain/cafe", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/domain/hotel", "github.com/dua-os/dua-os-be/internal/domain/cafe")
	assertNoImport(t, "internal/app/cafedef", "github.com/dua-os/dua-os-be/internal/app/assistant")
	assertNoImport(t, "internal/app/cafedef", "github.com/dua-os/dua-os-be/internal/app/cash")
	assertNoImport(t, "internal/app/cafedef", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafeqr", "github.com/dua-os/dua-os-be/internal/app/assistant")
	assertNoImport(t, "internal/app/cafeqr", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafeqr", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafeintake", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/cafeintake", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafeintake", "github.com/dua-os/dua-os-be/internal/app/assistant")
	assertNoImport(t, "internal/app/cafeintake", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/cafeintake")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/task")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/report")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/app/assistant")
	assertNoImport(t, "internal/app/cafehandover", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/app/cafeintake")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/app/cafehandover")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/app/assistant")
	assertNoImport(t, "internal/app/cafecheck", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/app/cafecheck")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/app/cash")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/app/approval")
	assertNoImport(t, "internal/app/cafepos", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/cafeprep", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/cafeprep", "github.com/dua-os/dua-os-be/internal/app/cafedef")
	assertNoImport(t, "internal/app/cafeprep", "github.com/dua-os/dua-os-be/internal/app/cafepos")
	assertNoImport(t, "internal/app/cafeprep", "github.com/dua-os/dua-os-be/internal/app/cafecheck")
	assertNoImport(t, "internal/app/cafeprep", "github.com/dua-os/dua-os-be/internal/domain/hotel")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafepos")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafeprep")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/cafepos")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/cafeprep")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafecheck")
	assertNoImport(t, "internal/domain/cafe", "github.com/dua-os/dua-os-be/internal/domain/finance")
	assertNoImport(t, "internal/app/task", "github.com/dua-os/dua-os-be/internal/app/staff")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/staff")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/task")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/decision")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafeqr")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafehandover")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/cafeintake")
	assertNoImport(t, "internal/app/brief", "github.com/dua-os/dua-os-be/internal/app/report")
	assertNoImport(t, "internal/app/decision", "github.com/dua-os/dua-os-be/internal/app/task")
	assertNoImport(t, "internal/app/decision", "github.com/dua-os/dua-os-be/internal/app/staff")
	assertNoImport(t, "internal/app/decision", "github.com/dua-os/dua-os-be/internal/app/brief")
	assertNoImport(t, "internal/app/report", "github.com/dua-os/dua-os-be/internal/app/task")
	assertNoImport(t, "internal/app/report", "github.com/dua-os/dua-os-be/internal/app/staff")
	assertNoImport(t, "internal/app/report", "github.com/dua-os/dua-os-be/internal/app/decision")
	assertNoImport(t, "internal/app/report", "github.com/dua-os/dua-os-be/internal/app/brief")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/task")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/staff")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/cash")
	assertNoImport(t, "internal/app/dashboard", "github.com/dua-os/dua-os-be/internal/app/approval")
	assertNoImport(t, "internal/app/cash", "github.com/dua-os/dua-os-be/internal/app/approval")
	assertNoImport(t, "internal/app/approval", "github.com/dua-os/dua-os-be/internal/app/cash")
	assertNoImport(t, "internal/domain/finance", "github.com/dua-os/dua-os-be/internal/domain/work")
	assertNoImport(t, "internal/domain/opsreport", "github.com/dua-os/dua-os-be/internal/domain/work")
	assertNoImport(t, "internal/domain/opsreport", "github.com/dua-os/dua-os-be/internal/domain/executive")
	assertNoImport(t, "internal/app/auth", "github.com/dua-os/dua-os-be/internal/platform/authn")
	assertNoImport(t, "internal/app/staff", "github.com/dua-os/dua-os-be/internal/platform/authn")
}

func TestHTTPHandlersStaySmall(t *testing.T) {
	root := findModuleRoot(t)
	dir := filepath.Join(root, "internal/adapter/http")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Count(string(body), "\n") + 1
		if lines > 400 {
			t.Errorf("%s is %d lines (Guest-pro fitness limit 400)", e.Name(), lines)
		}
	}
}

func assertNoImport(t *testing.T, dir, forbidden string) {
	t.Helper()
	root := findModuleRoot(t)
	err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), `"`+forbidden) {
			t.Errorf("%s imports %s", path, forbidden)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}
