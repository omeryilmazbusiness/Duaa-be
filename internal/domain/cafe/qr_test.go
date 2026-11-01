package cafe_test

import (
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestTokenIsUnguessableAndStableLength(t *testing.T) {
	a, err := cafe.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := cafe.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) != cafe.TokenLen {
		t.Fatalf("tokens = %s %s", a, b)
	}
	if _, err := cafe.ParseToken(a); err != nil {
		t.Fatal(err)
	}
	if _, err := cafe.ParseToken("short"); err == nil {
		t.Fatal("short token must fail")
	}
}

func TestPublicURLNeverPointsAtAPI(t *testing.T) {
	u := cafe.PublicURL("http://localhost:3000/", "abcdefghijabcdefghijab")
	if u != "http://localhost:3000/q/abcdefghijabcdefghijab" {
		t.Fatalf("url=%s", u)
	}
}

func TestCodeRequiresSite(t *testing.T) {
	c := cafe.Code{WorkspaceID: id.New(), Token: "abcdefghijabcdefghijab", IssuedAt: time.Now().UTC()}
	if err := c.Validate(); err == nil {
		t.Fatal("site is required")
	}
}

func TestVoiceRequiresExplicitRating(t *testing.T) {
	v := cafe.Voice{WorkspaceID: id.New(), SiteID: id.New(), Rating: 0}
	if err := v.Validate(); err == nil {
		t.Fatal("zero rating must fail — do not invent five stars")
	}
	v.Rating = 5
	v.Comment = "  nice  "
	v.Normalize()
	if v.Comment != "nice" || v.Kind != cafe.VoiceReview {
		t.Fatalf("voice=%+v", v)
	}
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	if v.Unhappy() {
		t.Fatal("5 stars is not unhappy")
	}
	low := v
	low.Rating = 2
	if !low.Unhappy() {
		t.Fatal("2 stars is unhappy")
	}
}

func TestComplaintAndRequestDoNotInventStars(t *testing.T) {
	v := cafe.Voice{WorkspaceID: id.New(), SiteID: id.New(), Kind: cafe.VoiceComplaint, Comment: "tea cold"}
	v.Normalize()
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	if !v.Unhappy() {
		t.Fatal("a complaint is unhappy even without stars")
	}
	ask := cafe.Voice{WorkspaceID: id.New(), SiteID: id.New(), Kind: cafe.VoiceRequest, Rating: 0}
	if err := ask.Validate(); err == nil {
		t.Fatal("request without a note must fail")
	}
	ask.Comment = "extra water"
	if err := ask.Validate(); err != nil {
		t.Fatal(err)
	}
	if ask.Unhappy() {
		t.Fatal("a request is not unhappy by default")
	}
}

func TestTableNumberIsANameNotAToken(t *testing.T) {
	n, err := cafe.ParseTableNumber("  12a  ")
	if err != nil || n != "12a" {
		t.Fatalf("table=%q err=%v", n, err)
	}
	if _, err := cafe.ParseTableNumber(""); err == nil {
		t.Fatal("empty table must fail")
	}
	if _, err := cafe.ParseTableNumber("***"); err == nil {
		t.Fatal("symbols must fail")
	}
}

func TestDuplicateTableNumbersAreRejected(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.QRSites = []cafe.QRSite{
		{Label: "12", Area: cafe.AreaTable, Active: true},
		{Label: "12", Area: cafe.AreaTable, Active: true},
	}
	d.Normalize()
	if err := d.Validate(); err == nil {
		t.Fatal("two tables named 12 must fail")
	}
}
