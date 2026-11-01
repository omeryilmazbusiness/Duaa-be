package httpadapter_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/adapter/memory"
	"github.com/dua-os/dua-os-be/internal/domain/identity"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

func TestCafeDefinitionSeedThenPersist(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	path := "/v1/workspaces/" + h.seed.CafeID.String() + "/cafe/definition"

	rec := get(h.handler, path, token)
	if rec.Code != 200 {
		t.Fatalf("GET seed %d %s", rec.Code, rec.Body.String())
	}
	var seed map[string]any
	decode(t, rec, &seed)
	if seed["displayName"] != "Duaa Cafe" || seed["timezone"] != "Asia/Baghdad" {
		t.Fatalf("seed identity = %+v", seed)
	}
	if seed["persisted"] != false {
		t.Fatal("GET must not invent a saved document")
	}
	if seed["posStatus"] != "unknown" {
		t.Fatalf("POS must stay unknown until workshop, got %v", seed["posStatus"])
	}
	comp := seed["completeness"].(map[string]any)
	if comp["readyForReviews"] != false {
		t.Fatal("empty workshop is not review-ready")
	}

	body := completeCafeBody("Duaa Cafe Karada")
	saved := put(h.handler, path, body, token)
	if saved.Code != 200 {
		t.Fatalf("PUT %d %s", saved.Code, saved.Body.String())
	}
	var view map[string]any
	decode(t, saved, &view)
	if view["persisted"] != true {
		t.Fatal("PUT must persist")
	}
	if view["displayName"] != "Duaa Cafe Karada" {
		t.Fatalf("name=%v", view["displayName"])
	}
	comp = view["completeness"].(map[string]any)
	if comp["readyForReviews"] != true {
		t.Fatalf("complete workshop must be ready: %s", saved.Body.String())
	}
	if len(view["branches"].([]any)) != 1 || len(view["qrSites"].([]any)) != 1 {
		t.Fatalf("collections = %s", saved.Body.String())
	}

	ws, err := h.store.GetWorkspace(t.Context(), h.seed.CafeID)
	if err != nil || ws.Name != "Duaa Cafe Karada" || ws.Timezone != "Asia/Baghdad" {
		t.Fatalf("workspace sync = %+v err=%v", ws, err)
	}

	again := get(h.handler, path, token)
	var reloaded map[string]any
	decode(t, again, &reloaded)
	if reloaded["displayName"] != "Duaa Cafe Karada" || reloaded["persisted"] != true {
		t.Fatalf("reload = %s", again.Body.String())
	}
}

func TestCafeDefinitionRBACAndHotel(t *testing.T) {
	h := newHarness(t)
	gm := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/definition"

	mgr := post(h.handler, "/v1/workspaces/"+cafeID+"/staff/department-managers", map[string]string{
		"firstName": "Layla", "lastName": "Kitchen",
		"email": "layla-def@cafe.test", "password": "Kitchen123",
		"staffDepartment": "KITCHEN",
	}, gm)
	if mgr.Code != 201 {
		t.Fatalf("mgr %d %s", mgr.Code, mgr.Body.String())
	}
	mgrLogin := post(h.handler, "/v1/auth/login", map[string]string{
		"email": "layla-def@cafe.test", "password": "Kitchen123",
	}, "")
	var mgrSession wrapLogin
	decode(t, mgrLogin, &mgrSession)

	if rec := get(h.handler, path, mgrSession.Token); rec.Code != 200 {
		t.Fatalf("manager GET %d %s", rec.Code, rec.Body.String())
	}
	if rec := put(h.handler, path, completeCafeBody("Duaa Cafe"), mgrSession.Token); rec.Code != 403 {
		t.Fatalf("manager PUT %d %s", rec.Code, rec.Body.String())
	}

	emp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Barista",
		"staffDepartment": "BAR", "employeeNumber": "2042",
	}, gm)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "2042",
	}, "")
	var staff wrapLogin
	decode(t, staffLogin, &staff)
	if rec := get(h.handler, path, staff.Token); rec.Code != 403 {
		t.Fatalf("staff GET %d %s", rec.Code, rec.Body.String())
	}

	hotel, err := tenancy.NewWorkspace(h.seed.OrganizationID, tenancy.KindHotel, "Pilot Hotel", "pilot-hotel", "UTC", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.CreateWorkspace(t.Context(), hotel); err != nil {
		t.Fatal(err)
	}
	if err := h.store.CreateMembership(t.Context(), identity.Membership{
		ID: id.New(), WorkspaceID: hotel.ID, UserID: id.MustParse("33333333-3333-4333-8333-333333333333"),
		Role: identity.RoleManager, IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	pick := post(h.handler, "/v1/auth/login/select", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword, "workspaceId": hotel.ID.String(),
	}, "")
	var hotelSession wrapLogin
	decode(t, pick, &hotelSession)
	if rec := get(h.handler, "/v1/workspaces/"+hotel.ID.String()+"/cafe/definition", hotelSession.Token); rec.Code != 422 {
		t.Fatalf("hotel GET %d %s", rec.Code, rec.Body.String())
	}
}

func TestCafeDefinitionRejectsLivePOS(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	body := completeCafeBody("Duaa Cafe")
	body["posStatus"] = "connected"
	body["posVendor"] = "Foodics"
	rec := put(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/cafe/definition", body, token)
	if rec.Code != 422 {
		t.Fatalf("connected POS %d %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	decode(t, rec, &payload)
	errObj := payload["error"].(map[string]any)
	if errObj["code"] != "pos_not_connected" {
		t.Fatalf("code=%v body=%s", errObj["code"], rec.Body.String())
	}
}

func TestCafeDefinitionSurvivesMemoryReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cafe-definition.json")
	h := newHarness(t)
	token := gmToken(t, h)
	if err := h.store.AttachCafePersist(path); err != nil {
		t.Fatal(err)
	}
	body := completeCafeBody("Duaa Cafe")
	rec := put(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/cafe/definition", body, token)
	if rec.Code != 200 {
		t.Fatalf("PUT %d %s", rec.Code, rec.Body.String())
	}

	b := memory.New()
	if err := b.AttachCafePersist(path); err != nil {
		t.Fatal(err)
	}
	got, err := b.GetCafeDefinition(t.Context(), h.seed.CafeID)
	if err != nil || got.DisplayName != "Duaa Cafe" || got.Currency != "IQD" || len(got.Branches) != 1 {
		t.Fatalf("reloaded = %+v err=%v", got, err)
	}
}

func completeCafeBody(name string) map[string]any {
	return map[string]any{
		"displayName": name,
		"currency":    "IQD",
		"timezone":    "Asia/Baghdad",
		"posVendor":   "",
		"posStatus":   "none",
		"branches":    []map[string]any{{"name": "Karada", "address": "", "active": true}},
		"qrSites":     []map[string]any{{"label": "Garden 4", "area": "garden", "active": true}},
		"shifts":      []map[string]any{{"name": "Morning", "start": "08:00", "end": "16:00"}},
		"sla": map[string]any{
			"ackMinutes": 15, "resolveMinutes": 120,
			"afterHoursMessage": "Call the duty manager.", "compensationLimit": 0,
			"compensationCurrency": "",
		},
	}
}

func TestCafeDefinitionJSONContract(t *testing.T) {
	raw, err := json.Marshal(completeCafeBody("Duaa Cafe"))
	if err != nil || len(raw) == 0 {
		t.Fatal(err)
	}
}
