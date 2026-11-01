// Package staffscope ports Guest-pro's GM / department-manager / personnel hierarchy
// onto a vertical catalog so cafe, hotel and supply share one rule engine.
package staffscope

import (
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
)

type Kind string

const (
	GeneralManager     Kind = "general_manager"
	DepartmentManager  Kind = "department_manager"
	FloorOps           Kind = "floor_ops"
	StaffPersonnel     Kind = "staff_personnel"
)

type Actor struct {
	Role       string
	Department string
}

func FromMembership(m identity.Membership) Actor {
	return Actor{Role: m.Role, Department: m.Department}
}

func Resolve(actor Actor, cat tenancy.Catalog) Kind {
	if actor.Role == identity.RoleManager {
		if actor.Department != "" && tenancy.ContainsDept(cat.DepartmentManagerDepartments(), tenancy.Department(actor.Department)) {
			return DepartmentManager
		}
		return GeneralManager
	}
	if actor.Role == identity.RolePersonnel {
		if actor.Department != "" && tenancy.Department(actor.Department) == cat.FloorOpsDepartment() {
			return FloorOps
		}
		return StaffPersonnel
	}
	return StaffPersonnel
}

func IsGeneralManager(actor Actor, cat tenancy.Catalog) bool {
	return Resolve(actor, cat) == GeneralManager
}

func IsAnyManager(actor Actor, cat tenancy.Catalog) bool {
	k := Resolve(actor, cat)
	return k == GeneralManager || k == DepartmentManager
}

func DepartmentScope(actor Actor, cat tenancy.Catalog) string {
	if Resolve(actor, cat) == DepartmentManager {
		return actor.Department
	}
	return ""
}

func CanManageStaff(actor Actor, cat tenancy.Catalog) bool {
	return IsAnyManager(actor, cat)
}

func CanManageStaffMember(actor Actor, member Actor, cat tenancy.Catalog) bool {
	if !CanManageStaff(actor, cat) {
		return false
	}
	if member.Role != identity.RolePersonnel {
		return false
	}
	switch Resolve(actor, cat) {
	case GeneralManager:
		return true
	case DepartmentManager:
		return member.Department == actor.Department
	default:
		return false
	}
}

func VisibleTabs(scope Kind, cat tenancy.Catalog) []tenancy.Tab {
	switch scope {
	case GeneralManager:
		return cat.GMTabs()
	case DepartmentManager:
		return cat.DepartmentManagerTabs()
	case FloorOps:
		return cat.FloorOpsTabs()
	default:
		return nil
	}
}

func DefaultTab(scope Kind, cat tenancy.Catalog) tenancy.Tab {
	tabs := VisibleTabs(scope, cat)
	if len(tabs) == 0 {
		return ""
	}
	return tabs[0]
}

func CanAccessTab(scope Kind, cat tenancy.Catalog, tab tenancy.Tab) bool {
	return tenancy.ContainsTab(VisibleTabs(scope, cat), tab)
}

func IsEmployeePortal(actor Actor, cat tenancy.Catalog) bool {
	if actor.Role != identity.RolePersonnel || actor.Department == "" {
		return false
	}
	return tenancy.ContainsDept(cat.EmployeeNumberDepartments(), tenancy.Department(actor.Department))
}

func Label(scope Kind, department string) string {
	switch scope {
	case GeneralManager:
		return "General Manager"
	case DepartmentManager:
		if department != "" {
			return department + " Manager"
		}
		return "Department Manager"
	case FloorOps:
		return "Floor"
	default:
		if department != "" {
			return department
		}
		return "Staff"
	}
}
