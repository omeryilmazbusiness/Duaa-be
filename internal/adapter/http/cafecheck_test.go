package httpadapter_test

import "testing"

func TestCafeChecklistEmptyThenTickRecordedLine(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/checklist"
	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var first map[string]any
	decode(t, empty, &first)
	if first["hasList"] != false {
		t.Fatalf("must not invent a list: %s", empty.Body.String())
	}

	if rec := put(h.handler, path, map[string]any{
		"items": []map[string]string{{"kind": "maintenance", "title": "photo fridge"}},
	}, token); rec.Code != 200 {
		t.Fatalf("maintenance drop %d %s", rec.Code, rec.Body.String())
	}
	quiet := get(h.handler, path, token)
	var afterDrop map[string]any
	decode(t, quiet, &afterDrop)
	if afterDrop["hasList"] != false {
		t.Fatalf("maintenance must not become a line: %s", quiet.Body.String())
	}

	saved := put(h.handler, path, map[string]any{
		"items": []map[string]string{
			{"kind": "open", "title": "chairs down"},
			{"kind": "close", "title": "safe locked"},
		},
	}, token)
	if saved.Code != 200 {
		t.Fatalf("save %d %s", saved.Code, saved.Body.String())
	}
	var view map[string]any
	decode(t, saved, &view)
	if view["hasList"] != true {
		t.Fatalf("saved=%s", saved.Body.String())
	}
	items, _ := view["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items=%s", saved.Body.String())
	}
	open := items[0].(map[string]any)
	ticked := post(h.handler, path+"/toggle", map[string]string{
		"businessDate": view["businessDate"].(string),
		"itemId":       open["id"].(string),
	}, token)
	if ticked.Code != 200 {
		t.Fatalf("tick %d %s", ticked.Code, ticked.Body.String())
	}
	var live map[string]any
	decode(t, ticked, &live)
	op := live["open"].(map[string]any)
	cl := live["close"].(map[string]any)
	if op["done"] != float64(1) || cl["remaining"] != float64(1) {
		t.Fatalf("pulse=%s", ticked.Body.String())
	}
	if rec := post(h.handler, path+"/toggle", map[string]string{"itemId": "00000000-0000-4000-8000-000000000099"}, token); rec.Code != 422 {
		t.Fatalf("unknown line %d %s", rec.Code, rec.Body.String())
	}
}

func TestCafeChecklistStaffForbidden(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	emp := post(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Bar", "staffDepartment": "BAR", "employeeNumber": "4106",
	}, token)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	staff := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "4106",
	}, "")
	var session wrapLogin
	decode(t, staff, &session)
	if rec := get(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/cafe/checklist", session.Token); rec.Code != 403 {
		t.Fatalf("staff %d %s", rec.Code, rec.Body.String())
	}
}
