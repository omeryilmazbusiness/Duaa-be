package httpadapter

import (
	"net/http"
	"time"

	appcafedef "github.com/dua-os/dua-os-be/internal/app/cafedef"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const maxCafeBody = 64 << 10

type cafeDefinitionView struct {
	WorkspaceID  string            `json:"workspaceId"`
	DisplayName  string            `json:"displayName"`
	Currency     string            `json:"currency"`
	Timezone     string            `json:"timezone"`
	POSVendor    string            `json:"posVendor"`
	POSStatus    string            `json:"posStatus"`
	Branches     []cafeBranchView  `json:"branches"`
	QRSites      []cafeQRView      `json:"qrSites"`
	Shifts       []cafeShiftView   `json:"shifts"`
	SLA          cafeSLAView       `json:"sla"`
	Completeness cafe.Completeness `json:"completeness"`
	CanWrite     bool              `json:"canWrite"`
	Persisted    bool              `json:"persisted"`
	UpdatedAt    string            `json:"updatedAt,omitempty"`
	UpdatedBy    string            `json:"updatedBy,omitempty"`
}

type cafeBranchView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Active  bool   `json:"active"`
}

type cafeQRView struct {
	ID       string `json:"id"`
	BranchID string `json:"branchId,omitempty"`
	Label    string `json:"label"`
	Area     string `json:"area"`
	Active   bool   `json:"active"`
}

type cafeShiftView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type cafeSLAView struct {
	AckMinutes           int    `json:"ackMinutes"`
	ResolveMinutes       int    `json:"resolveMinutes"`
	AfterHoursMessage    string `json:"afterHoursMessage"`
	CompensationLimit    int64  `json:"compensationLimit"`
	CompensationCurrency string `json:"compensationCurrency"`
}

func (a API) cafeGate(w http.ResponseWriter, r *http.Request) (*appcafedef.Service, bool) {
	if a.Cafe == nil {
		httpx.WriteError(w, errx.New("cafe_unavailable", "cafe definition is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxCafeBody)
	return a.Cafe, true
}

func (a API) getCafeDefinition(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.cafeGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	view, err := svc.Get(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toCafeView(view))
}

func (a API) putCafeDefinition(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.cafeGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var body struct {
		DisplayName string           `json:"displayName"`
		Currency    string           `json:"currency"`
		Timezone    string           `json:"timezone"`
		POSVendor   string           `json:"posVendor"`
		POSStatus   string           `json:"posStatus"`
		Branches    []cafeBranchView `json:"branches"`
		QRSites     []cafeQRView     `json:"qrSites"`
		Shifts      []cafeShiftView  `json:"shifts"`
		SLA         cafeSLAView      `json:"sla"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.WriteError(w, err)
		return
	}
	view, err := svc.Save(r.Context(), ac, appcafedef.SaveInput{
		DisplayName: body.DisplayName,
		Currency:    body.Currency,
		Timezone:    body.Timezone,
		POSVendor:   body.POSVendor,
		POSStatus:   body.POSStatus,
		Branches:    parseBranches(body.Branches),
		QRSites:     parseQRSites(body.QRSites),
		Shifts:      parseShifts(body.Shifts),
		SLA: cafe.SLA{
			AckMinutes: body.SLA.AckMinutes, ResolveMinutes: body.SLA.ResolveMinutes,
			AfterHoursMessage: body.SLA.AfterHoursMessage, CompensationLimit: body.SLA.CompensationLimit,
			CompensationCurrency: body.SLA.CompensationCurrency,
		},
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if a.QR != nil {
		if err := a.QR.EnsureWorkspace(r.Context(), ac.Workspace.ID, view.Definition.DisplayName, view.Definition.Timezone); err != nil {
			httpx.WriteError(w, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, toCafeView(view))
}

func toCafeView(v appcafedef.View) cafeDefinitionView {
	d := v.Definition
	out := cafeDefinitionView{
		WorkspaceID: d.WorkspaceID.String(),
		DisplayName: d.DisplayName,
		Currency:    d.Currency,
		Timezone:    d.Timezone,
		POSVendor:   d.POSVendor,
		POSStatus:   string(d.POSStatus),
		Branches:    make([]cafeBranchView, 0, len(d.Branches)),
		QRSites:     make([]cafeQRView, 0, len(d.QRSites)),
		Shifts:      make([]cafeShiftView, 0, len(d.Shifts)),
		SLA: cafeSLAView{
			AckMinutes: d.SLA.AckMinutes, ResolveMinutes: d.SLA.ResolveMinutes,
			AfterHoursMessage: d.SLA.AfterHoursMessage, CompensationLimit: d.SLA.CompensationLimit,
			CompensationCurrency: d.SLA.CompensationCurrency,
		},
		Completeness: v.Completeness,
		CanWrite:     v.CanWrite,
		Persisted:    v.Persisted,
	}
	if !d.UpdatedAt.IsZero() {
		out.UpdatedAt = d.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if d.UpdatedBy != id.Zero() {
		out.UpdatedBy = d.UpdatedBy.String()
	}
	for _, b := range d.Branches {
		out.Branches = append(out.Branches, cafeBranchView{ID: b.ID.String(), Name: b.Name, Address: b.Address, Active: b.Active})
	}
	for _, q := range d.QRSites {
		row := cafeQRView{ID: q.ID.String(), Label: q.Label, Area: string(q.Area), Active: q.Active}
		if q.BranchID != id.Zero() {
			row.BranchID = q.BranchID.String()
		}
		out.QRSites = append(out.QRSites, row)
	}
	for _, s := range d.Shifts {
		out.Shifts = append(out.Shifts, cafeShiftView{ID: s.ID.String(), Name: s.Name, Start: s.Start, End: s.End})
	}
	return out
}

func parseBranches(in []cafeBranchView) []cafe.Branch {
	out := make([]cafe.Branch, 0, len(in))
	for _, b := range in {
		row := cafe.Branch{Name: b.Name, Address: b.Address, Active: b.Active}
		if bid, err := id.Parse(b.ID); err == nil {
			row.ID = bid
		}
		out = append(out, row)
	}
	return out
}

func parseQRSites(in []cafeQRView) []cafe.QRSite {
	out := make([]cafe.QRSite, 0, len(in))
	for _, q := range in {
		row := cafe.QRSite{Label: q.Label, Area: cafe.Area(q.Area), Active: q.Active}
		if qid, err := id.Parse(q.ID); err == nil {
			row.ID = qid
		}
		if q.BranchID != "" {
			if bid, err := id.Parse(q.BranchID); err == nil {
				row.BranchID = bid
			}
		}
		out = append(out, row)
	}
	return out
}

func parseShifts(in []cafeShiftView) []cafe.Shift {
	out := make([]cafe.Shift, 0, len(in))
	for _, s := range in {
		row := cafe.Shift{Name: s.Name, Start: s.Start, End: s.End}
		if sid, err := id.Parse(s.ID); err == nil {
			row.ID = sid
		}
		out = append(out, row)
	}
	return out
}
