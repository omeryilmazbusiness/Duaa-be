package cafedoc

import (
	"encoding/json"
	"time"

	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// Document is the durable shape of a cafe definition. Adapters share it so
// memory persist and Postgres JSONB stay byte-compatible.
type Document struct {
	WorkspaceID string      `json:"workspaceId"`
	DisplayName string      `json:"displayName"`
	Currency    string      `json:"currency"`
	Timezone    string      `json:"timezone"`
	POSVendor   string      `json:"posVendor"`
	POSStatus   string      `json:"posStatus"`
	Branches    []branchDoc `json:"branches"`
	QRSites     []qrDoc     `json:"qrSites"`
	Shifts      []shiftDoc  `json:"shifts"`
	SLA         slaDoc      `json:"sla"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	UpdatedBy   string      `json:"updatedBy,omitempty"`
}

type branchDoc struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Active  bool   `json:"active"`
}

type qrDoc struct {
	ID       string `json:"id"`
	BranchID string `json:"branchId,omitempty"`
	Label    string `json:"label"`
	Area     string `json:"area"`
	Active   bool   `json:"active"`
}

type shiftDoc struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type slaDoc struct {
	AckMinutes           int    `json:"ackMinutes"`
	ResolveMinutes       int    `json:"resolveMinutes"`
	AfterHoursMessage    string `json:"afterHoursMessage"`
	CompensationLimit    int64  `json:"compensationLimit"`
	CompensationCurrency string `json:"compensationCurrency"`
}

func FromDomain(d cafe.Definition) Document {
	doc := Document{
		WorkspaceID: d.WorkspaceID.String(),
		DisplayName: d.DisplayName,
		Currency:    d.Currency,
		Timezone:    d.Timezone,
		POSVendor:   d.POSVendor,
		POSStatus:   string(d.POSStatus),
		Branches:    make([]branchDoc, 0, len(d.Branches)),
		QRSites:     make([]qrDoc, 0, len(d.QRSites)),
		Shifts:      make([]shiftDoc, 0, len(d.Shifts)),
		SLA: slaDoc{
			AckMinutes: d.SLA.AckMinutes, ResolveMinutes: d.SLA.ResolveMinutes,
			AfterHoursMessage: d.SLA.AfterHoursMessage, CompensationLimit: d.SLA.CompensationLimit,
			CompensationCurrency: d.SLA.CompensationCurrency,
		},
		UpdatedAt: d.UpdatedAt,
	}
	if d.UpdatedBy != id.Zero() {
		doc.UpdatedBy = d.UpdatedBy.String()
	}
	for _, b := range d.Branches {
		doc.Branches = append(doc.Branches, branchDoc{ID: b.ID.String(), Name: b.Name, Address: b.Address, Active: b.Active})
	}
	for _, q := range d.QRSites {
		row := qrDoc{ID: q.ID.String(), Label: q.Label, Area: string(q.Area), Active: q.Active}
		if q.BranchID != id.Zero() {
			row.BranchID = q.BranchID.String()
		}
		doc.QRSites = append(doc.QRSites, row)
	}
	for _, s := range d.Shifts {
		doc.Shifts = append(doc.Shifts, shiftDoc{ID: s.ID.String(), Name: s.Name, Start: s.Start, End: s.End})
	}
	return doc
}

func ToDomain(doc Document) (cafe.Definition, error) {
	wid, err := id.Parse(doc.WorkspaceID)
	if err != nil {
		return cafe.Definition{}, err
	}
	d := cafe.Definition{
		WorkspaceID: wid,
		DisplayName: doc.DisplayName,
		Currency:    doc.Currency,
		Timezone:    doc.Timezone,
		POSVendor:   doc.POSVendor,
		POSStatus:   cafe.POSStatus(doc.POSStatus),
		Branches:    make([]cafe.Branch, 0, len(doc.Branches)),
		QRSites:     make([]cafe.QRSite, 0, len(doc.QRSites)),
		Shifts:      make([]cafe.Shift, 0, len(doc.Shifts)),
		SLA: cafe.SLA{
			AckMinutes: doc.SLA.AckMinutes, ResolveMinutes: doc.SLA.ResolveMinutes,
			AfterHoursMessage: doc.SLA.AfterHoursMessage, CompensationLimit: doc.SLA.CompensationLimit,
			CompensationCurrency: doc.SLA.CompensationCurrency,
		},
		UpdatedAt: doc.UpdatedAt,
	}
	if doc.UpdatedBy != "" {
		if uid, err := id.Parse(doc.UpdatedBy); err == nil {
			d.UpdatedBy = uid
		}
	}
	for _, b := range doc.Branches {
		row := cafe.Branch{Name: b.Name, Address: b.Address, Active: b.Active}
		if bid, err := id.Parse(b.ID); err == nil {
			row.ID = bid
		}
		d.Branches = append(d.Branches, row)
	}
	for _, q := range doc.QRSites {
		row := cafe.QRSite{Label: q.Label, Area: cafe.Area(q.Area), Active: q.Active}
		if qid, err := id.Parse(q.ID); err == nil {
			row.ID = qid
		}
		if q.BranchID != "" {
			if bid, err := id.Parse(q.BranchID); err == nil {
				row.BranchID = bid
			}
		}
		d.QRSites = append(d.QRSites, row)
	}
	for _, s := range doc.Shifts {
		row := cafe.Shift{Name: s.Name, Start: s.Start, End: s.End}
		if sid, err := id.Parse(s.ID); err == nil {
			row.ID = sid
		}
		d.Shifts = append(d.Shifts, row)
	}
	return d, nil
}

func Marshal(d cafe.Definition) ([]byte, error) {
	return json.Marshal(FromDomain(d))
}

func Unmarshal(raw []byte) (cafe.Definition, error) {
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return cafe.Definition{}, err
	}
	return ToDomain(doc)
}
