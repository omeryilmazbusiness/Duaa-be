package cafe_test

import (
	"strings"
	"testing"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestEmptySeedIsValidAndIncomplete(t *testing.T) {
	d := cafe.Seed(id.MustParse("22222222-2222-4222-8222-222222222222"), "Duaa Cafe", "Asia/Baghdad")
	d.Normalize()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	c := d.Completeness()
	if c.ReadyForReviews {
		t.Fatal("seed must not be review-ready; branches, QR and SLA are unknown")
	}
	if !c.HasName || !c.HasTimezone || c.HasCurrency || c.HasBranch || c.HasQRSite || c.HasSLA {
		t.Fatalf("seed completeness = %+v", c)
	}
	wantMissing := []string{"currency", "branch", "qr_site", "sla"}
	if strings.Join(c.Missing, ",") != strings.Join(wantMissing, ",") {
		t.Fatalf("missing = %v", c.Missing)
	}
}

func TestReadyForReviewsNeedsTheWorkshopFacts(t *testing.T) {
	d := completeDefinition()
	d.Normalize()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	if !d.Completeness().ReadyForReviews {
		t.Fatalf("complete definition must be ready, missing=%v", d.Completeness().Missing)
	}
	d.SLA.AckMinutes = 0
	if d.Completeness().ReadyForReviews {
		t.Fatal("SLA acknowledgement is required for reviews")
	}
}

func TestRejectsLivePOSWithoutAdapter(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "Asia/Baghdad")
	d.POSStatus = "connected"
	d.POSVendor = "Foodics"
	d.Normalize()
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("connected POS must be rejected, err=%v", err)
	}
}

func TestNamedPOSRequiresVendor(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.POSStatus = cafe.POSNamed
	if err := d.Validate(); err == nil {
		t.Fatal("named POS without vendor must fail")
	}
	d.POSVendor = "Foodics"
	d.Normalize()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNonePOSClearsVendor(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.POSStatus = cafe.POSNone
	d.POSVendor = " leftover "
	d.Normalize()
	if d.POSVendor != "" {
		t.Fatalf("vendor=%q", d.POSVendor)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestVendorPromotesUnknownToNamed(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.POSVendor = "Square"
	d.Normalize()
	if d.POSStatus != cafe.POSNamed {
		t.Fatalf("status=%s", d.POSStatus)
	}
}

func TestCurrencyAndTimezone(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "Not/AZone")
	if err := d.Validate(); err == nil {
		t.Fatal("bad timezone must fail")
	}
	d.Timezone = "Asia/Baghdad"
	d.Currency = "iqd"
	d.Normalize()
	if d.Currency != "IQD" {
		t.Fatalf("currency=%s", d.Currency)
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Currency = "IQ"
	if err := d.Validate(); err == nil {
		t.Fatal("short currency must fail")
	}
}

func TestOvernightShiftIsValid(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "Asia/Baghdad")
	d.Shifts = []cafe.Shift{{Name: "Late", Start: "22:00", End: "06:00"}}
	d.Normalize()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.Shifts[0].Start = "25:00"
	if err := d.Validate(); err == nil {
		t.Fatal("invalid clock must fail")
	}
}

func TestQRMustBelongToKnownBranch(t *testing.T) {
	wid := id.New()
	d := cafe.Seed(wid, "Duaa Cafe", "UTC")
	d.Branches = []cafe.Branch{{Name: "Karada"}, {Name: "Mansour"}}
	d.QRSites = []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden}}
	d.Normalize()
	if err := d.Validate(); err == nil {
		t.Fatal("QR without branch among two locations must fail")
	}
	d.QRSites[0].BranchID = d.Branches[0].ID
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	d.QRSites[0].BranchID = id.New()
	if err := d.Validate(); err == nil {
		t.Fatal("unknown branch id must fail")
	}
}

func TestSingleBranchBindsQR(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.Branches = []cafe.Branch{{Name: "Karada", Active: true}}
	d.QRSites = []cafe.QRSite{{Label: "Counter", Area: cafe.AreaCounter, Active: true}}
	d.Normalize()
	if d.QRSites[0].BranchID != d.Branches[0].ID {
		t.Fatal("single branch must bind QR sites")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSLAOrderAndCompensation(t *testing.T) {
	d := cafe.Seed(id.New(), "Duaa Cafe", "UTC")
	d.SLA = cafe.SLA{AckMinutes: 30, ResolveMinutes: 10}
	if err := d.Validate(); err == nil {
		t.Fatal("resolve < ack must fail")
	}
	d.SLA = cafe.SLA{AckMinutes: 10, ResolveMinutes: 30, CompensationLimit: 25000}
	if err := d.Validate(); err == nil {
		t.Fatal("compensation without currency must fail")
	}
	d.SLA.CompensationCurrency = "iqd"
	d.Normalize()
	if d.SLA.CompensationCurrency != "IQD" {
		t.Fatal("compensation currency must normalize")
	}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInactiveRowsDoNotCountForReviews(t *testing.T) {
	d := completeDefinition()
	d.Branches[0].Active = false
	d.QRSites[0].Active = false
	c := d.Completeness()
	if c.HasBranch || c.HasQRSite || c.ReadyForReviews {
		t.Fatalf("inactive rows must not count: %+v", c)
	}
}

func TestCloneDoesNotAliasSlices(t *testing.T) {
	d := completeDefinition()
	cp := d.Clone()
	cp.Branches[0].Name = "Changed"
	if d.Branches[0].Name == "Changed" {
		t.Fatal("clone must copy branches")
	}
}

func completeDefinition() cafe.Definition {
	d := cafe.Seed(id.MustParse("22222222-2222-4222-8222-222222222222"), "Duaa Cafe", "Asia/Baghdad")
	d.Currency = "IQD"
	d.Branches = []cafe.Branch{{Name: "Karada", Active: true}}
	d.QRSites = []cafe.QRSite{{Label: "Garden 4", Area: cafe.AreaGarden, Active: true}}
	d.Shifts = []cafe.Shift{{Name: "Morning", Start: "08:00", End: "16:00"}}
	d.SLA = cafe.SLA{AckMinutes: 15, ResolveMinutes: 120, CompensationCurrency: "IQD"}
	d.Normalize()
	return d
}
