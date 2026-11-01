package supply

import "github.com/dua-os/dua-os-be/internal/domain/tenancy"

// Catalog is a reserved Ana Akhdar vertical. Registered so workspace.kind=supply
// is a first-class tenant, not a later schema rewrite.
const (
	Sales      tenancy.Department = "SALES"
	Warehouse  tenancy.Department = "WAREHOUSE"
	Delivery   tenancy.Department = "DELIVERY"
	Accounting tenancy.Department = "FINANCIAL_ACCOUNTING"
)

type Catalog struct{}

func (Catalog) Kind() tenancy.Kind { return tenancy.KindSupply }

func (Catalog) Departments() []tenancy.Department {
	return []tenancy.Department{Sales, Warehouse, Delivery, Accounting}
}

func (Catalog) DepartmentManagerDepartments() []tenancy.Department {
	return []tenancy.Department{Sales, Warehouse, Delivery, Accounting}
}

func (Catalog) EmployeeNumberDepartments() []tenancy.Department {
	return []tenancy.Department{Warehouse, Delivery, Accounting}
}

func (Catalog) EmailLoginDepartments() []tenancy.Department {
	return []tenancy.Department{Sales}
}

func (Catalog) FloorOpsDepartment() tenancy.Department { return Sales }

func (Catalog) GMTabs() []tenancy.Tab {
	return []tenancy.Tab{tenancy.TabToday, tenancy.TabOps, tenancy.TabDashboard}
}

func (Catalog) DepartmentManagerTabs() []tenancy.Tab {
	return []tenancy.Tab{
		tenancy.TabOverview, tenancy.TabTasksStaff, tenancy.TabTasksDetail, tenancy.TabRoutines, tenancy.TabTeam,
	}
}

func (Catalog) FloorOpsTabs() []tenancy.Tab {
	return []tenancy.Tab{tenancy.TabRequests}
}

func (Catalog) ExecutiveSignals() []tenancy.SignalKind {
	return []tenancy.SignalKind{tenancy.SignalLateStaff}
}

func (Catalog) DashboardModules() []tenancy.DashboardModule {
	return []tenancy.DashboardModule{tenancy.ModulePulse, tenancy.ModuleApprovals}
}
