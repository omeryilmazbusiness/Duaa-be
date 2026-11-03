package httpadapter_test

import "testing"

func TestStockBindCatalogAndCountOnly(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/stock"

	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var first map[string]any
	decode(t, empty, &first)
	if first["status"] != "unavailable" || first["itemCount"] != float64(0) {
		t.Fatalf("empty stock must stay unavailable: %s", empty.Body.String())
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Zain", "lastName": "Floor", "staffDepartment": "BAR", "employeeNumber": "2099",
	}, token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	empLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "2099",
	}, "")
	var emp wrapLogin
	decode(t, empLogin, &emp)

	if put(h.handler, path+"/catalog", map[string]any{
		"items": []map[string]any{{"name": "Oat milk", "station": "bar", "unit": "ml", "par": 3000}},
	}, emp.Token).Code != 403 {
		t.Fatal("staff must not edit catalog")
	}

	saved := put(h.handler, path+"/catalog", map[string]any{
		"items": []map[string]any{{"name": "Oat milk", "station": "bar", "unit": "ml", "par": 3000, "active": true}},
	}, token)
	if saved.Code != 200 {
		t.Fatalf("catalog %d %s", saved.Code, saved.Body.String())
	}
	var book map[string]any
	decode(t, saved, &book)
	if book["status"] != "live" {
		t.Fatalf("catalog must go live: %s", saved.Body.String())
	}
	items, _ := book["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items: %v", items)
	}
	item, _ := items[0].(map[string]any)
	itemID, _ := item["id"].(string)

	if post(h.handler, path+"/adjustments", map[string]any{
		"itemId": itemID, "reason": "count", "onHand": 1800,
	}, emp.Token).Code != 403 {
		t.Fatal("staff personnel must not record stock")
	}

	counted := post(h.handler, path+"/adjustments", map[string]any{
		"itemId": itemID, "reason": "count", "onHand": 1800,
	}, token)
	if counted.Code != 201 {
		t.Fatalf("count %d %s", counted.Code, counted.Body.String())
	}
	var live map[string]any
	decode(t, counted, &live)
	liveItems, _ := live["items"].([]any)
	row, _ := liveItems[0].(map[string]any)
	if row["onHand"] != float64(1800) || row["level"] != float64(60) || live["lowCount"] != float64(1) {
		t.Fatalf("counted view: %s", counted.Body.String())
	}
}
