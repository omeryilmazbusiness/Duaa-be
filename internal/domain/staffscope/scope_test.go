package staffscope_test

import (
	"testing"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
)

func TestCafeHierarchyMatchesGuestPro(t *testing.T) {
	cat := cafe.Catalog{}

	if got := staffscope.Resolve(staffscope.Actor{Role: "manager"}, cat); got != staffscope.GeneralManager {
		t.Fatalf("gm: %s", got)
	}
	if got := staffscope.Resolve(staffscope.Actor{Role: "manager", Department: "KITCHEN"}, cat); got != staffscope.DepartmentManager {
		t.Fatalf("dept mgr: %s", got)
	}
	if got := staffscope.Resolve(staffscope.Actor{Role: "personnel", Department: "SERVICE"}, cat); got != staffscope.FloorOps {
		t.Fatalf("service floor: %s", got)
	}
	if got := staffscope.Resolve(staffscope.Actor{Role: "personnel", Department: "BAR"}, cat); got != staffscope.StaffPersonnel {
		t.Fatalf("barista: %s", got)
	}
}

func TestDepartmentManagerCannotManageOtherDept(t *testing.T) {
	cat := cafe.Catalog{}
	actor := staffscope.Actor{Role: "manager", Department: "KITCHEN"}
	ok := staffscope.CanManageStaffMember(actor, staffscope.Actor{Role: "personnel", Department: "KITCHEN"}, cat)
	if !ok {
		t.Fatal("kitchen manager should manage kitchen staff")
	}
	ok = staffscope.CanManageStaffMember(actor, staffscope.Actor{Role: "personnel", Department: "BAR"}, cat)
	if ok {
		t.Fatal("kitchen manager must not manage bar staff")
	}
	ok = staffscope.CanManageStaffMember(actor, staffscope.Actor{Role: "manager", Department: ""}, cat)
	if ok {
		t.Fatal("dept manager must not manage GM")
	}
}

func TestCafeGMTabsHaveNoReservations(t *testing.T) {
	cat := cafe.Catalog{}
	tabs := staffscope.VisibleTabs(staffscope.GeneralManager, cat)
	if tenancy.ContainsTab(tabs, tenancy.TabReservations) {
		t.Fatal("cafe GM must not see hotel reservations")
	}
	if !tenancy.ContainsTab(tabs, tenancy.TabFeedback) {
		t.Fatal("cafe GM must see feedback")
	}
	if staffscope.DefaultTab(staffscope.GeneralManager, cat) != tenancy.TabDashboard {
		t.Fatal("GM lands on dashboard")
	}
}

func TestHotelCatalogPreservesGuestProTabs(t *testing.T) {
	cat := hotel.Catalog{}
	tabs := staffscope.VisibleTabs(staffscope.GeneralManager, cat)
	want := []tenancy.Tab{tenancy.TabToday, tenancy.TabOps, tenancy.TabDashboard, tenancy.TabReservations, tenancy.TabFeedback}
	if len(tabs) != len(want) {
		t.Fatalf("tabs=%v", tabs)
	}
	for i := range want {
		if tabs[i] != want[i] {
			t.Fatalf("tab %d: %s != %s", i, tabs[i], want[i])
		}
	}
	if staffscope.Resolve(staffscope.Actor{Role: "personnel", Department: "RECEPTION"}, cat) != staffscope.FloorOps {
		t.Fatal("hotel reception should be floor ops")
	}
}

func TestEmployeePortalDepartments(t *testing.T) {
	cat := cafe.Catalog{}
	if !staffscope.IsEmployeePortal(staffscope.Actor{Role: "personnel", Department: "BAR"}, cat) {
		t.Fatal("bar staff uses 4-digit portal")
	}
	if staffscope.IsEmployeePortal(staffscope.Actor{Role: "personnel", Department: "SERVICE"}, cat) {
		t.Fatal("service uses email login, not employee portal")
	}
}
