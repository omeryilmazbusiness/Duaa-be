package hotel

import "github.com/dua-os/dua-os-be/internal/domain/tenancy"

// Catalog mirrors Guest-pro hotel departments and GM/manager tabs.
// Hotel HTTP surfaces are not wired in v1 — the catalog is registered so a hotel
// workspace can be created later without a schema fork.
const (
	Housekeeping tenancy.Department = "HOUSEKEEPING"
	Bellman      tenancy.Department = "BELLMAN"
	Reception    tenancy.Department = "RECEPTION"
	Restaurant   tenancy.Department = "RESTAURANT"
	Kitchen      tenancy.Department = "KITCHEN"
	Accounting   tenancy.Department = "FINANCIAL_ACCOUNTING"
	Security     tenancy.Department = "SECURITY"
	Maintenance  tenancy.Department = "MAINTENANCE"
	Marketing    tenancy.Department = "MARKETING"
	SpaGym       tenancy.Department = "SPA_GYM"
)

type Catalog struct{}

func (Catalog) Kind() tenancy.Kind { return tenancy.KindHotel }

func (Catalog) Departments() []tenancy.Department {
	return []tenancy.Department{
		Housekeeping, Bellman, Reception, Restaurant, Kitchen,
		Accounting, Security, Maintenance, Marketing, SpaGym,
	}
}

func (Catalog) DepartmentManagerDepartments() []tenancy.Department {
	return []tenancy.Department{
		Housekeeping, Bellman, Reception, Kitchen,
		Accounting, Security, Maintenance, Marketing, SpaGym,
	}
}

func (Catalog) EmployeeNumberDepartments() []tenancy.Department {
	return []tenancy.Department{
		Housekeeping, Bellman, Kitchen, Accounting, Security, Maintenance, Marketing, SpaGym,
	}
}

func (Catalog) EmailLoginDepartments() []tenancy.Department {
	return []tenancy.Department{Reception, Restaurant}
}

func (Catalog) FloorOpsDepartment() tenancy.Department { return Reception }

func (Catalog) GMTabs() []tenancy.Tab {
	return []tenancy.Tab{
		tenancy.TabToday,
		tenancy.TabOps,
		tenancy.TabDashboard,
		tenancy.TabReservations,
		tenancy.TabFeedback,
	}
}

func (Catalog) DepartmentManagerTabs() []tenancy.Tab {
	return []tenancy.Tab{
		tenancy.TabOverview,
		tenancy.TabTasksStaff,
		tenancy.TabTasksDetail,
		tenancy.TabRoutines,
		tenancy.TabTeam,
	}
}

func (Catalog) FloorOpsTabs() []tenancy.Tab {
	return []tenancy.Tab{tenancy.TabGuests, tenancy.TabLiveChat, tenancy.TabRequests}
}

func (Catalog) ExecutiveSignals() []tenancy.SignalKind {
	return []tenancy.SignalKind{tenancy.SignalLateStaff, tenancy.SignalCustomerVoice, tenancy.SignalReservations}
}

func (Catalog) DashboardModules() []tenancy.DashboardModule {
	return []tenancy.DashboardModule{tenancy.ModulePulse, tenancy.ModuleOccupancy, tenancy.ModuleApprovals}
}
