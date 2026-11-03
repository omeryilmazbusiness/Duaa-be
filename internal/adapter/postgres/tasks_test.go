package postgres

import (
	"strings"
	"testing"
)

func TestTaskSelectCastsNullableDate(t *testing.T) {
	// Postgres rejects coalesce(date_col, '') — must cast first.
	if !strings.Contains(taskSelect, "coalesce(source_date::text,'')") {
		t.Fatalf("taskSelect must cast source_date before coalesce with '': %s", taskSelect)
	}
	if strings.Contains(taskSelect, "coalesce(source_date,'')") {
		t.Fatal("taskSelect must not coalesce date with empty string directly")
	}
}
