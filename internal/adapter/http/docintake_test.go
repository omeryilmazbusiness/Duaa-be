package httpadapter_test

import "testing"

func TestWorkspaceDocumentSimplifierAndDedup(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	path := "/v1/workspaces/" + h.seed.CafeID.String() + "/documents"

	csv := "station,item,cost\nbar,Oat milk,18000\nkitchen,Beans,42000\n"
	created := post(h.handler, path, map[string]any{
		"fileName": "morning_supply.csv",
		"content":  csv,
		"locale":   "tr",
	}, token)
	if created.Code != 201 {
		t.Fatalf("ingest %d %s", created.Code, created.Body.String())
	}
	var first map[string]any
	decode(t, created, &first)
	if first["dataCoveragePct"] != float64(100) || first["replayed"] == true {
		t.Fatalf("unexpected doc=%s", created.Body.String())
	}

	replay := post(h.handler, path, map[string]any{
		"fileName": "morning_supply.csv",
		"content":  csv,
	}, token)
	if replay.Code != 201 {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	var dup map[string]any
	decode(t, replay, &dup)
	if dup["replayed"] != true {
		t.Fatalf("expected replayed=true, got %s", replay.Body.String())
	}

	listed := get(h.handler, path, token)
	if listed.Code != 200 {
		t.Fatalf("list %d %s", listed.Code, listed.Body.String())
	}
	var out struct {
		Documents []map[string]any `json:"documents"`
	}
	decode(t, listed, &out)
	if len(out.Documents) != 1 {
		t.Fatalf("expected 1 deduplicated document, got %d", len(out.Documents))
	}
}
