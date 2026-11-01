package httpadapter_test

import "testing"

func TestCafeIntakeEmptyThenUpsertSameDay(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/intake"
	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var first map[string]any
	decode(t, empty, &first)
	if first["persisted"] != false || first["closeNote"] != "" {
		t.Fatalf("must not invent a close: %s", empty.Body.String())
	}

	if rec := put(h.handler, path, map[string]string{"shiftName": "Night", "wasteNote": "bread leftover"}, token); rec.Code != 422 {
		t.Fatalf("invented shift %d %s", rec.Code, rec.Body.String())
	}

	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", completeCafeBody("Duaa Cafe"), token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}
	saved := put(h.handler, path, map[string]string{
		"shiftName": "Morning", "closeNote": "quiet close", "wasteNote": "bread leftover",
	}, token)
	if saved.Code != 200 {
		t.Fatalf("save %d %s", saved.Code, saved.Body.String())
	}
	var view map[string]any
	decode(t, saved, &view)
	if view["persisted"] != true || view["shiftName"] != "Morning" || view["wasteNote"] != "bread leftover" {
		t.Fatalf("saved=%s", saved.Body.String())
	}
	again := put(h.handler, path, map[string]string{
		"businessDate": view["businessDate"].(string),
		"shiftName":    "Morning",
		"closeNote":    "quiet close",
		"wasteNote":    "bread leftover",
		"incidentNote": "glass broke",
	}, token)
	var upsert map[string]any
	decode(t, again, &upsert)
	if upsert["id"] != view["id"] || upsert["incidentNote"] != "glass broke" {
		t.Fatalf("upsert=%s", again.Body.String())
	}
	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/intakes", token)
	var items wrapItems
	decode(t, list, &items)
	if len(items.Items) != 1 {
		t.Fatalf("list=%s", list.Body.String())
	}
}

func TestCafeIntakeHotelRejected(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	emp := post(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Bar", "staffDepartment": "BAR", "employeeNumber": "4101",
	}, token)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	staff := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "4101",
	}, "")
	var session wrapLogin
	decode(t, staff, &session)
	if rec := get(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/cafe/intake", session.Token); rec.Code != 403 {
		t.Fatalf("staff %d %s", rec.Code, rec.Body.String())
	}
}
