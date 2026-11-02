package httpadapter_test

import "testing"

func TestPrepBindCAF03RecordedClocksOnly(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/prep"

	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var first map[string]any
	decode(t, empty, &first)
	if first["status"] != "unavailable" {
		t.Fatalf("empty prep must stay unavailable: %s", empty.Body.String())
	}

	brief := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", token)
	var morning map[string]any
	decode(t, brief, &morning)
	for _, raw := range morning["sources"].([]any) {
		src, _ := raw.(map[string]any)
		if src["kind"] == "prep_time" && src["status"] == "live" {
			t.Fatal("empty prep must not be a live source")
		}
	}
	for _, raw := range morning["cards"].([]any) {
		card, _ := raw.(map[string]any)
		if card["kind"] == "prep_time" {
			t.Fatal("empty prep must not raise a card")
		}
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Zain", "lastName": "Floor", "staffDepartment": "BAR", "employeeNumber": "1088",
	}, token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	empLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1088",
	}, "")
	var emp wrapLogin
	decode(t, empLogin, &emp)
	if post(h.handler, path+"/tickets", map[string]any{
		"station": "kitchen", "ticketRef": "K-1", "tableLabel": "12",
	}, emp.Token).Code != 403 {
		t.Fatal("staff must not record prep")
	}

	opened := post(h.handler, path+"/tickets", map[string]any{
		"station": "kitchen", "ticketRef": "K-1", "tableLabel": "12",
	}, token)
	if opened.Code != 201 {
		t.Fatalf("open %d %s", opened.Code, opened.Body.String())
	}
	var live map[string]any
	decode(t, opened, &live)
	if live["status"] != "live" || live["waitingCount"] != float64(1) {
		t.Fatalf("open view=%v", live)
	}
	if post(h.handler, path+"/tickets", map[string]any{
		"station": "kitchen", "ticketRef": "k-1", "tableLabel": "12",
	}, token).Code != 409 {
		t.Fatal("duplicate ticket")
	}
	tickets, _ := live["tickets"].([]any)
	row, _ := tickets[0].(map[string]any)
	id, _ := row["id"].(string)
	if post(h.handler, path+"/"+id+"/serve", map[string]any{}, token).Code != 422 {
		t.Fatal("serve before ready")
	}
	ready := post(h.handler, path+"/"+id+"/ready", map[string]any{}, token)
	if ready.Code != 200 {
		t.Fatalf("ready %d %s", ready.Code, ready.Body.String())
	}
	waiting := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", token)
	var after map[string]any
	decode(t, waiting, &after)
	found := false
	for _, raw := range after["cards"].([]any) {
		card, _ := raw.(map[string]any)
		if card["kind"] == "prep_time" && card["count"] == float64(1) {
			found = true
		}
	}
	if !found {
		t.Fatalf("waiting ticket must raise a Today card: %s", waiting.Body.String())
	}
	served := post(h.handler, path+"/"+id+"/serve", map[string]any{}, token)
	if served.Code != 200 {
		t.Fatalf("serve %d %s", served.Code, served.Body.String())
	}
	var done map[string]any
	decode(t, served, &done)
	if done["servedCount"] != float64(1) || done["waitingCount"] != float64(0) {
		t.Fatalf("served=%v", done)
	}
}
