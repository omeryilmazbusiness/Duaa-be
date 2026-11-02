package rbac_test

import (
	"testing"

	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
)

func TestEmployeeCannotManageStaff(t *testing.T) {
	if rbac.Allows(staffscope.StaffPersonnel, rbac.ManageStaff) {
		t.Fatal("employee must not manage staff")
	}
	if rbac.Allows(staffscope.StaffPersonnel, rbac.ManageWorkspace) {
		t.Fatal("employee must not admin workspace")
	}
	if !rbac.Allows(staffscope.StaffPersonnel, rbac.SubmitReport) {
		t.Fatal("employee submits shift reports")
	}
	if rbac.Allows(staffscope.StaffPersonnel, rbac.ViewReports) {
		t.Fatal("employee must not read the manager report inbox")
	}
}

func TestOnlyGMTouchesFinance(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.CloseCash) || !rbac.Allows(staffscope.GeneralManager, rbac.ActApproval) || !rbac.Allows(staffscope.GeneralManager, rbac.RecordPOS) {
		t.Fatal("GM closes cash, records POS and acts on spend packs")
	}
	if rbac.Allows(staffscope.DepartmentManager, rbac.CloseCash) || rbac.Allows(staffscope.DepartmentManager, rbac.ActApproval) || rbac.Allows(staffscope.DepartmentManager, rbac.RecordPOS) {
		t.Fatal("department manager must not close till, record POS or approve spend")
	}
	if !rbac.Allows(staffscope.DepartmentManager, rbac.SubmitApproval) {
		t.Fatal("department manager may submit a spend pack")
	}
	if rbac.Allows(staffscope.StaffPersonnel, rbac.ViewFinance) {
		t.Fatal("employee must not read finance")
	}
}

func TestManagersReadReportInbox(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.ViewReports) {
		t.Fatal("GM reads employee reports")
	}
	if !rbac.Allows(staffscope.FloorOps, rbac.ViewReports) {
		t.Fatal("floor reads employee reports")
	}
}

func TestOnlyGMAdminsWorkspace(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.ManageWorkspace) {
		t.Fatal("GM admins workspace")
	}
	if rbac.Allows(staffscope.DepartmentManager, rbac.ManageWorkspace) {
		t.Fatal("dept manager is not workspace admin")
	}
}

func TestOnlyGMUsesAIAssistant(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.UseAI) {
		t.Fatal("GM uses the AI assistant")
	}
	for _, scope := range []staffscope.Kind{staffscope.DepartmentManager, staffscope.FloorOps, staffscope.StaffPersonnel} {
		if rbac.Allows(scope, rbac.UseAI) {
			t.Fatalf("%s must not use the AI assistant", scope)
		}
	}
}

func TestFloorRecordsPrepTimes(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.RecordPrep) || !rbac.Allows(staffscope.DepartmentManager, rbac.RecordPrep) || !rbac.Allows(staffscope.FloorOps, rbac.RecordPrep) {
		t.Fatal("managers record kitchen wait")
	}
	if rbac.Allows(staffscope.StaffPersonnel, rbac.RecordPrep) {
		t.Fatal("employee must not record prep times")
	}
}

func TestManagersReadCafeDefinition(t *testing.T) {
	if !rbac.Allows(staffscope.GeneralManager, rbac.ViewCafe) {
		t.Fatal("GM reads cafe definition")
	}
	if !rbac.Allows(staffscope.DepartmentManager, rbac.ViewCafe) {
		t.Fatal("department manager reads cafe definition")
	}
	if !rbac.Allows(staffscope.FloorOps, rbac.ViewCafe) {
		t.Fatal("floor reads cafe definition")
	}
	if rbac.Allows(staffscope.StaffPersonnel, rbac.ViewCafe) {
		t.Fatal("employee must not read cafe definition")
	}
}

func TestPostLoginSurface(t *testing.T) {
	if rbac.PostLoginSurface(staffscope.StaffPersonnel, nil) != "staff_portal" {
		t.Fatal("personnel lands on staff portal")
	}
	if rbac.PostLoginSurface(staffscope.GeneralManager, nil) != "manager" {
		t.Fatal("GM lands on manager shell")
	}
}
