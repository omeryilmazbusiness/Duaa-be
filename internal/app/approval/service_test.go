package approval_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/app/approval"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fake struct {
	rows map[id.ID]finance.Approval
}

func (f *fake) CreateApproval(_ context.Context, a finance.Approval) error {
	if f.rows == nil {
		f.rows = map[id.ID]finance.Approval{}
	}
	f.rows[a.ID] = a
	return nil
}
func (f *fake) GetApproval(_ context.Context, aid id.ID) (finance.Approval, error) {
	a, ok := f.rows[aid]
	if !ok {
		return finance.Approval{}, errx.NotFound("approval_not_found", "missing")
	}
	return a, nil
}
func (f *fake) ListApprovals(_ context.Context, workspaceID id.ID) ([]finance.Approval, error) {
	out := make([]finance.Approval, 0)
	for _, a := range f.rows {
		if a.WorkspaceID == workspaceID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fake) UpdateApproval(_ context.Context, a finance.Approval) error {
	if _, ok := f.rows[a.ID]; !ok {
		return errx.NotFound("approval_not_found", "missing")
	}
	f.rows[a.ID] = a
	return nil
}

func TestSubmitRejectsDuplicatePending(t *testing.T) {
	wid := id.New()
	svc := approval.New(&fake{}, clock.Frozen{T: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	first, err := svc.Submit(t.Context(), ac, "Milk invoice", "Al-Nahrain", "photo", 250000)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.Status != finance.StatusOpen {
		t.Fatalf("first=%+v", first)
	}
	if _, err := svc.Submit(t.Context(), ac, "MILK invoice", "al-nahrain", "other", 250000); err == nil || !errors.Is(err, errx.ErrConflict) {
		t.Fatalf("duplicate pending: %v", err)
	}
}

func TestStaffCannotSubmitOrAct(t *testing.T) {
	wid := id.New()
	svc := approval.New(&fake{}, clock.System{})
	ac := gm(wid)
	ac.Scope = staffscope.StaffPersonnel
	if _, err := svc.Submit(t.Context(), ac, "Milk", "S", "photo", 1); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff submit: %v", err)
	}
	if _, err := svc.Act(t.Context(), ac, id.New().String(), finance.ActionApprove, nil); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff act: %v", err)
	}
	if _, err := svc.AttachEvidence(t.Context(), ac, id.New().String(), "photo"); err == nil || !errors.Is(err, errx.ErrForbidden) {
		t.Fatalf("staff evidence: %v", err)
	}
}

func TestAttachThenApproveDoesNotChangeAmount(t *testing.T) {
	wid := id.New()
	svc := approval.New(&fake{}, clock.Frozen{T: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)})
	ac := gm(wid)
	pack, err := svc.Submit(t.Context(), ac, "Milk invoice", "Al-Nahrain", "", 250000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Act(t.Context(), ac, pack.ID, finance.ActionApprove, nil); err == nil {
		t.Fatal("approve without evidence")
	}
	asked, err := svc.Act(t.Context(), ac, pack.ID, finance.ActionRequestEvidence, nil)
	if err != nil || asked.Status != finance.StatusNeedsEvidence {
		t.Fatalf("request=%+v err=%v", asked, err)
	}
	if _, err := svc.Act(t.Context(), ac, pack.ID, finance.ActionApprove, nil); err == nil {
		t.Fatal("approve while waiting for evidence")
	}
	ready, err := svc.AttachEvidence(t.Context(), ac, pack.ID, "photo-ref")
	if err != nil || ready.Status != finance.StatusOpen || ready.Version != 2 {
		t.Fatalf("attach=%+v err=%v", ready, err)
	}
	done, err := svc.Act(t.Context(), ac, pack.ID, finance.ActionApprove, nil)
	if err != nil || done.Status != finance.StatusApproved || done.Amount != 250000 {
		t.Fatalf("approve must not pay or change amount: %+v err=%v", done, err)
	}
}

func TestReviseReopensAndRejectsDuplicateFingerprint(t *testing.T) {
	wid := id.New()
	svc := approval.New(&fake{}, clock.System{})
	ac := gm(wid)
	first, err := svc.Submit(t.Context(), ac, "Milk invoice", "Al-Nahrain", "photo", 250000)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Submit(t.Context(), ac, "Milk invoice", "Al-Nahrain", "photo", 260000)
	if err != nil {
		t.Fatal(err)
	}
	amt := int64(250000)
	if _, err := svc.Act(t.Context(), ac, second.ID, "", &amt); err == nil || !errors.Is(err, errx.ErrConflict) {
		t.Fatalf("revise onto a pending twin: %v", err)
	}
	if _, err := svc.Act(t.Context(), ac, first.ID, finance.ActionApprove, nil); err != nil {
		t.Fatal(err)
	}
}

func gm(wid id.ID) actor.Context {
	uid := id.New()
	return actor.Context{
		Principal: ctxkey.Principal{UserID: uid.String(), WorkspaceID: wid.String(), Role: identity.RoleManager},
		Workspace: tenancy.Workspace{ID: wid, Kind: tenancy.KindCafe, Name: "Duaa Cafe", Timezone: "Asia/Baghdad"},
		Catalog:   cafe.Catalog{},
		Scope:     staffscope.GeneralManager,
	}
}
