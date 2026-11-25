package inbox_test

import (
	"testing"

	"github.com/dua-os/dua-os-be/internal/domain/inbox"
)

func TestNormalizeDraft(t *testing.T) {
	d, err := inbox.NormalizeDraft([]string{" Guest@Example.com ", "guest@example.com", "ops@hotel.com"}, "  Room request ", "Please confirm.")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.To) != 2 || d.To[0] != "guest@example.com" || d.Subject != "Room request" {
		t.Fatalf("%+v", d)
	}
}

func TestNormalizeDraftRejectsBad(t *testing.T) {
	if _, err := inbox.NormalizeDraft(nil, "Hi", "Body"); err == nil {
		t.Fatal("expected missing recipient")
	}
	if _, err := inbox.NormalizeDraft([]string{"not-an-email"}, "Hi", "Body"); err == nil {
		t.Fatal("expected invalid recipient")
	}
	if _, err := inbox.ParseComposeLang("ar"); err == nil {
		t.Fatal("compose must be en or tr")
	}
	if _, err := inbox.ParseCategory("guest"); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewOf(t *testing.T) {
	short := inbox.PreviewOf("  hello   world ")
	if short != "hello world" {
		t.Fatalf("%q", short)
	}
}
