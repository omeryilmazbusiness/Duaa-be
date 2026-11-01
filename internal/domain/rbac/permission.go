package rbac

import (
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
)

// Permission is a server-evaluated entitlement (Duaa spec CORE ENTITLEMENTS).
type Permission string

const (
	ViewDashboard   Permission = "dashboard.view"
	ManageWorkspace Permission = "workspace.admin"
	ManageStaff     Permission = "staff.manage"
	ViewTasks       Permission = "task.view"
	ManageTasks     Permission = "task.assign"
	ViewReviews     Permission = "review.view"
	ManageReviews   Permission = "review.manage"
	ViewReports     Permission = "report.view"
	SubmitReport    Permission = "report.submit"
	ViewFinance     Permission = "finance.view"
	CloseCash       Permission = "cash.close"
	SubmitApproval  Permission = "approval.submit"
	ActApproval     Permission = "approval.act"
	UseAI           Permission = "ai.use"
	ViewCafe        Permission = "cafe.view"
)

func Allows(scope staffscope.Kind, perm Permission) bool {
	switch perm {
	case ViewDashboard:
		return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager || scope == staffscope.FloorOps
	case ManageWorkspace:
		return scope == staffscope.GeneralManager
	case ManageStaff, ViewTasks, ManageTasks:
		return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager
	case ViewReviews, ManageReviews:
		return scope == staffscope.GeneralManager || scope == staffscope.FloorOps
	case ViewReports:
		return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager || scope == staffscope.FloorOps
	case SubmitReport:
		return scope == staffscope.DepartmentManager || scope == staffscope.StaffPersonnel || scope == staffscope.FloorOps || scope == staffscope.GeneralManager
	case ViewFinance, CloseCash, ActApproval:
		return scope == staffscope.GeneralManager
	case SubmitApproval:
		return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager
	case UseAI:
		// The AI assistant is a GM-only surface. Setup uses ManageWorkspace.
		return scope == staffscope.GeneralManager
	case ViewCafe:
		return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager || scope == staffscope.FloorOps
	default:
		return false
	}
}

func Require(scope staffscope.Kind, perm Permission) bool {
	return Allows(scope, perm)
}

// ManagerSurface reports whether this scope uses the Guest-pro manager shell (not employee portal).
func ManagerSurface(scope staffscope.Kind) bool {
	return scope == staffscope.GeneralManager || scope == staffscope.DepartmentManager || scope == staffscope.FloorOps
}

func PostLoginSurface(scope staffscope.Kind, _ tenancy.Catalog) string {
	if scope == staffscope.StaffPersonnel {
		return "staff_portal"
	}
	return "manager"
}
