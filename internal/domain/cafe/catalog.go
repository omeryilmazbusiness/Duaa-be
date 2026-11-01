package cafe

import "github.com/dua-os/dua-os-be/internal/domain/tenancy"

// Departments for the cafe vertical. Mapped from Guest-pro hotel depts onto cafe ops.
const (
	Kitchen     tenancy.Department = "KITCHEN"
	Bar         tenancy.Department = "BAR"
	Service     tenancy.Department = "SERVICE" // floor / reception analog
	Cleaning    tenancy.Department = "CLEANING"
	Maintenance tenancy.Department = "MAINTENANCE"
	Accounting  tenancy.Department = "FINANCIAL_ACCOUNTING"
)

type Catalog struct{}

func (Catalog) Kind() tenancy.Kind { return tenancy.KindCafe }

func (Catalog) Departments() []tenancy.Department {
	return []tenancy.Department{Kitchen, Bar, Service, Cleaning, Maintenance, Accounting}
}

func (Catalog) DepartmentManagerDepartments() []tenancy.Department {
	return []tenancy.Department{Kitchen, Bar, Service, Cleaning, Maintenance, Accounting}
}

func (Catalog) EmployeeNumberDepartments() []tenancy.Department {
	return []tenancy.Department{Kitchen, Bar, Cleaning, Maintenance, Accounting}
}

func (Catalog) EmailLoginDepartments() []tenancy.Department {
	return []tenancy.Department{Service}
}

func (Catalog) FloorOpsDepartment() tenancy.Department { return Service }

func (Catalog) GMTabs() []tenancy.Tab {
	// Guest-pro GM: today, ops, dashboard, reservations, feedback.
	// Cafe has no rooms/reservations; feedback (QR reviews) stays.
	return []tenancy.Tab{
		tenancy.TabDashboard,
		tenancy.TabToday,
		tenancy.TabOps,
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
	return []tenancy.Tab{tenancy.TabRequests, tenancy.TabFeedback}
}

func (Catalog) ExecutiveSignals() []tenancy.SignalKind {
	return []tenancy.SignalKind{
		tenancy.SignalHandover, tenancy.SignalLateStaff, tenancy.SignalCustomerVoice,
		tenancy.SignalBreakdown, tenancy.SignalDailyClose, tenancy.SignalCashVariance,
	}
}

func (Catalog) DashboardModules() []tenancy.DashboardModule {
	return []tenancy.DashboardModule{
		tenancy.ModulePulse,
		tenancy.ModuleReports,
		tenancy.ModuleCash,
		tenancy.ModuleApprovals,
		tenancy.ModuleSales,
		tenancy.ModuleWaste,
	}
}
