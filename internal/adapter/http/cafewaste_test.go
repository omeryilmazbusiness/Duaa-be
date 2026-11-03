package httpadapter_test

import "testing"

func TestWasteBindCAF02RecordedGramsAndCostOnly(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/waste"

	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var first map[string]any
	decode(t, empty, &first)
	if first["status"] != "unavailable" || first["cost"] != float64(0) {
		t.Fatalf("empty waste must stay unavailable: %s", empty.Body.String())
	}

	dash := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/dashboard", token)
	var morning map[string]any
	decode(t, dash, &morning)
	waste, _ := morning["waste"].(map[string]any)
	if waste["status"] != "unavailable" {
		t.Fatalf("empty dashboard waste must stay unavailable: %v", waste)
	}
	for _, raw := range morning["tiles"].([]any) {
		tile, _ := raw.(map[string]any)
		if tile["module"] == "waste" && tile["status"] == "live" {
			t.Fatal("waste tile must not go live with no tickets")
		}
	}

	brief := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", token)
	var morningBrief map[string]any
	decode(t, brief, &morningBrief)
	for _, raw := range morningBrief["sources"].([]any) {
		src, _ := raw.(map[string]any)
		if src["kind"] == "waste" && src["status"] == "live" {
			t.Fatal("empty waste must not be a live source")
		}
	}
	for _, raw := range morningBrief["cards"].([]any) {
		card, _ := raw.(map[string]any)
		if card["kind"] == "waste" {
			t.Fatal("recorded waste must not nag as a card even later; empty must not raise one")
		}
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Zain", "lastName": "Floor", "staffDepartment": "BAR", "employeeNumber": "1099",
	}, token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	empLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1099",
	}, "")
	var emp wrapLogin
	decode(t, empLogin, &emp)
	if post(h.handler, path+"/tickets", map[string]any{
		"station": "kitchen", "item": "milk", "grams": 500, "cost": 8000,
	}, emp.Token).Code != 403 {
		t.Fatal("staff must not record waste")
	}

	if post(h.handler, path+"/tickets", map[string]any{
		"station": "kitchen", "item": "milk", "grams": 500, "cost": 0,
	}, token).Code != 422 {
		t.Fatal("zero cost")
	}

	pos := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/pos/tickets", map[string]any{
		"externalRef": "W-POS-1", "kind": "sale", "tender": "cash", "lines": 1, "gross": 12000, "discount": 0,
	}, token)
	if pos.Code != 201 {
		t.Fatalf("pos %d %s", pos.Code, pos.Body.String())
	}
	afterPOS := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/dashboard", token)
	var posDash map[string]any
	decode(t, afterPOS, &posDash)
	waste, _ = posDash["waste"].(map[string]any)
	if waste["status"] == "live" {
		t.Fatal("POS sales must not invent waste")
	}

	saved := put(h.handler, path+"/recipes", map[string]any{
		"recipes": []map[string]any{{
			"name": "Latte", "station": "bar",
			"lines": []map[string]any{{"title": "milk", "grams": 180}},
		}},
	}, token)
	if saved.Code != 200 {
		t.Fatalf("recipes %d %s", saved.Code, saved.Body.String())
	}
	var book map[string]any
	decode(t, saved, &book)
	recipes, _ := book["recipes"].([]any)
	if len(recipes) != 1 {
		t.Fatalf("recipes=%v", book)
	}
	recipe, _ := recipes[0].(map[string]any)
	recipeID, _ := recipe["id"].(string)

	opened := post(h.handler, path+"/tickets", map[string]any{
		"station": "bar", "item": "latte", "reason": "prep", "grams": 180, "cost": 2500, "recipeId": recipeID,
	}, token)
	if opened.Code != 201 {
		t.Fatalf("open %d %s", opened.Code, opened.Body.String())
	}
	var live map[string]any
	decode(t, opened, &live)
	if live["status"] != "live" || live["cost"] != float64(2500) || live["grams"] != float64(180) {
		t.Fatalf("live view=%v", live)
	}

	liveDash := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/dashboard", token)
	var after map[string]any
	decode(t, liveDash, &after)
	waste, _ = after["waste"].(map[string]any)
	if waste["status"] != "live" || waste["cost"] != float64(2500) {
		t.Fatalf("dashboard waste=%v", waste)
	}
	foundTile := false
	for _, raw := range after["tiles"].([]any) {
		tile, _ := raw.(map[string]any)
		if tile["module"] == "waste" && tile["status"] == "live" && tile["value"] == float64(2500) {
			foundTile = true
		}
	}
	if !foundTile {
		t.Fatalf("waste tile must go live from recorded cost: %s", liveDash.Body.String())
	}

	waiting := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", token)
	var afterBrief map[string]any
	decode(t, waiting, &afterBrief)
	liveSource := false
	for _, raw := range afterBrief["sources"].([]any) {
		src, _ := raw.(map[string]any)
		if src["kind"] == "waste" && src["status"] == "live" {
			liveSource = true
		}
	}
	if !liveSource {
		t.Fatalf("recorded waste must be a live source: %s", waiting.Body.String())
	}
	for _, raw := range afterBrief["cards"].([]any) {
		card, _ := raw.(map[string]any)
		if card["kind"] == "waste" {
			t.Fatal("recorded waste must not raise a decision card")
		}
	}
}
