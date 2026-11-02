package tenancy

// Department is a vertical-specific staff classification (HOUSEKEEPING, KITCHEN, …).
type Department string

// Tab is a management-shell screen id. Guest-pro FE navigation is driven by these ids.
type Tab string

// SignalKind is a morning-brief input. Catalogs opt in; missing adapters stay unavailable.
type SignalKind string

const (
	SignalLateStaff     SignalKind = "late_staff"
	SignalCustomerVoice SignalKind = "customer_voice"
	SignalReservations  SignalKind = "reservations"
	SignalCashVariance  SignalKind = "cash_variance"
	SignalHandover      SignalKind = "handover"
	SignalBreakdown     SignalKind = "breakdown"
	SignalDailyClose    SignalKind = "daily_close"
	SignalChecklist     SignalKind = "checklist"
	SignalPrepTime      SignalKind = "prep_time"
)

// DashboardModule is a GM pulse tile. Catalogs opt in; missing adapters stay unavailable.
type DashboardModule string

const (
	ModulePulse     DashboardModule = "pulse"
	ModuleReports   DashboardModule = "reports"
	ModuleCash      DashboardModule = "cash"
	ModuleApprovals DashboardModule = "approvals"
	ModuleSales     DashboardModule = "sales"
	ModuleWaste     DashboardModule = "waste"
	ModuleOccupancy DashboardModule = "occupancy"
)

const (
	TabToday        Tab = "today"
	TabOps          Tab = "ops"
	TabDashboard    Tab = "dashboard"
	TabOverview     Tab = "overview"
	TabGuests       Tab = "guests"
	TabReservations Tab = "reservations"
	TabRooms        Tab = "rooms"
	TabRequests     Tab = "requests"
	TabFeedback     Tab = "feedback"
	TabTeam         Tab = "team"
	TabTasksStaff   Tab = "tasks_staff"
	TabTasksDetail  Tab = "tasks_detail"
	TabRoutines     Tab = "routines"
	TabLiveChat     Tab = "live_chat"
)

// Catalog is the Open/Closed extension point for a vertical.
// Cafe is the first implementation; hotel/supply register the same contract later.
type Catalog interface {
	Kind() Kind
	Departments() []Department
	DepartmentManagerDepartments() []Department
	EmployeeNumberDepartments() []Department
	EmailLoginDepartments() []Department
	// FloorOpsDepartment is the reception analog (email login + limited ops surface).
	FloorOpsDepartment() Department
	GMTabs() []Tab
	DepartmentManagerTabs() []Tab
	FloorOpsTabs() []Tab
	// ExecutiveSignals is the Open/Closed list of morning-OS inputs for this vertical.
	ExecutiveSignals() []SignalKind
	// DashboardModules is the Open/Closed list of GM pulse tiles for this vertical.
	DashboardModules() []DashboardModule
}

func ContainsDept(list []Department, d Department) bool {
	for _, item := range list {
		if item == d {
			return true
		}
	}
	return false
}

func ContainsTab(list []Tab, t Tab) bool {
	for _, item := range list {
		if item == t {
			return true
		}
	}
	return false
}

func ContainsSignal(list []SignalKind, k SignalKind) bool {
	for _, item := range list {
		if item == k {
			return true
		}
	}
	return false
}
