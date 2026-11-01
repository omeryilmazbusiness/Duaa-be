package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func patchJSON(h http.Handler, path string, body any, token string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestQRIssuanceResolveAndVoice(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	body := completeCafeBody("Duaa Cafe")
	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", body, token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}

	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", token)
	if list.Code != 200 {
		t.Fatalf("list %d %s", list.Code, list.Body.String())
	}
	var codes wrapItems
	decode(t, list, &codes)
	if len(codes.Items) != 1 {
		t.Fatalf("want 1 code, got %s", list.Body.String())
	}
	qrToken := codes.Items[0]["token"].(string)
	if codes.Items[0]["publicPath"] != "/q/"+qrToken {
		t.Fatalf("path=%v", codes.Items[0]["publicPath"])
	}

	pub := get(h.handler, "/v1/public/qr/"+qrToken, "")
	if pub.Code != 200 {
		t.Fatalf("public %d %s", pub.Code, pub.Body.String())
	}
	var guest map[string]any
	decode(t, pub, &guest)
	if guest["cafeName"] != "Duaa Cafe" || guest["label"] != "Garden 4" || guest["reviewsOpen"] != true {
		t.Fatalf("guest=%s", pub.Body.String())
	}

	img := get(h.handler, "/v1/public/qr/"+qrToken+"/image", "")
	if img.Code != 200 || !strings.Contains(img.Body.String(), "<svg") {
		t.Fatalf("image %d %s", img.Code, img.Body.String())
	}

	created := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{
		"rating": 2, "comment": "tea was cold",
	}, "")
	if created.Code != 201 {
		t.Fatalf("voice %d %s", created.Code, created.Body.String())
	}

	inbox := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices", token)
	var voices wrapItems
	decode(t, inbox, &voices)
	if len(voices.Items) != 1 || voices.Items[0]["unhappy"] != true {
		t.Fatalf("inbox=%s", inbox.Body.String())
	}

	brief := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", token)
	var view map[string]any
	decode(t, brief, &view)
	found := false
	for _, raw := range view["sources"].([]any) {
		src := raw.(map[string]any)
		if src["kind"] == "customer_voice" {
			found = true
			if src["status"] != "live" {
				t.Fatalf("wired QR must be a live source: %v", src)
			}
		}
	}
	if !found {
		t.Fatal("customer voice source missing")
	}
}

func TestQRSubmitBlockedUntilWorkshopComplete(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	body := completeCafeBody("Duaa Cafe")
	body["sla"] = map[string]any{"ackMinutes": 0, "resolveMinutes": 0, "afterHoursMessage": "", "compensationLimit": 0, "compensationCurrency": ""}
	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", body, token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}
	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", token)
	var codes wrapItems
	decode(t, list, &codes)
	qrToken := codes.Items[0]["token"].(string)
	pub := get(h.handler, "/v1/public/qr/"+qrToken, "")
	var guest map[string]any
	decode(t, pub, &guest)
	if guest["reviewsOpen"] != false {
		t.Fatal("incomplete workshop must keep reviews closed")
	}
	rec := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{"rating": 5, "comment": "ok"}, "")
	if rec.Code != 422 || errCode(t, rec) != "reviews_not_ready" {
		t.Fatalf("submit %d %s", rec.Code, rec.Body.String())
	}
}

func TestQRRotateKillsOldToken(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", completeCafeBody("Duaa Cafe"), token)
	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", token)
	var codes wrapItems
	decode(t, list, &codes)
	siteID := codes.Items[0]["siteId"].(string)
	old := codes.Items[0]["token"].(string)
	rot := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr/"+siteID+"/rotate", map[string]any{}, token)
	if rot.Code != 200 {
		t.Fatalf("rotate %d %s", rot.Code, rot.Body.String())
	}
	if rec := get(h.handler, "/v1/public/qr/"+old, ""); rec.Code != 404 {
		t.Fatalf("old token %d %s", rec.Code, rec.Body.String())
	}
}

func TestIssueTableRenameKeepsTokenAndGuestIntents(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", completeCafeBody("Duaa Cafe"), token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}
	issued := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr/tables", map[string]string{"tableNumber": "12"}, token)
	if issued.Code != 200 {
		t.Fatalf("issue %d %s", issued.Code, issued.Body.String())
	}
	var code map[string]any
	decode(t, issued, &code)
	qrToken := code["token"].(string)
	siteID := code["siteId"].(string)
	if code["label"] != "12" || code["area"] != "table" {
		t.Fatalf("issued=%s", issued.Body.String())
	}
	again := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr/tables", map[string]string{"tableNumber": "12"}, token)
	var same map[string]any
	decode(t, again, &same)
	if same["token"] != qrToken {
		t.Fatal("reissue must not reprint the QR")
	}
	ren := patchJSON(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr/"+siteID, map[string]string{"tableNumber": "7"}, token)
	if ren.Code != 200 {
		t.Fatalf("rename %d %s", ren.Code, ren.Body.String())
	}
	var renamed map[string]any
	decode(t, ren, &renamed)
	if renamed["token"] != qrToken || renamed["label"] != "7" {
		t.Fatalf("rename=%s", ren.Body.String())
	}
	pub := get(h.handler, "/v1/public/qr/"+qrToken, "")
	var guest map[string]any
	decode(t, pub, &guest)
	if guest["label"] != "7" || guest["reviewsOpen"] != true {
		t.Fatalf("guest=%s", pub.Body.String())
	}
	ask := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{"kind": "request", "comment": "extra water"}, "")
	if ask.Code != 201 {
		t.Fatalf("request %d %s", ask.Code, ask.Body.String())
	}
	comp := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{"kind": "complaint", "comment": "tea was cold"}, "")
	if comp.Code != 201 {
		t.Fatalf("complaint %d %s", comp.Code, comp.Body.String())
	}
	inbox := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices", token)
	var voices wrapItems
	decode(t, inbox, &voices)
	if len(voices.Items) != 2 {
		t.Fatalf("inbox=%s", inbox.Body.String())
	}
}

func TestVoiceRecoveryAssignCompensateClose(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	body := completeCafeBody("Duaa Cafe")
	body["sla"] = map[string]any{
		"ackMinutes": 15, "resolveMinutes": 120, "afterHoursMessage": "Call the duty manager.",
		"compensationLimit": 20, "compensationCurrency": "IQD",
	}
	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", body, token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}
	emp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Noor", "lastName": "Floor", "staffDepartment": "BAR", "employeeNumber": "3101",
	}, token)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	var member map[string]any
	decode(t, emp, &member)
	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", token)
	var codes wrapItems
	decode(t, list, &codes)
	qrToken := codes.Items[0]["token"].(string)
	created := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{"rating": 2, "comment": "tea was cold"}, "")
	if created.Code != 201 {
		t.Fatalf("voice %d %s", created.Code, created.Body.String())
	}
	var voice map[string]any
	decode(t, created, &voice)
	if voice["needsRecovery"] != true || voice["recoveryStatus"] != "open" {
		t.Fatalf("open file=%s", created.Body.String())
	}
	happy := post(h.handler, "/v1/public/qr/"+qrToken, map[string]any{"rating": 5, "comment": "great"}, "")
	var five map[string]any
	decode(t, happy, &five)
	if five["needsRecovery"] == true || five["recoveryStatus"] != nil && five["recoveryStatus"] != "" {
		t.Fatalf("happy review must not invent a file: %s", happy.Body.String())
	}
	over := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices/"+voice["id"].(string)+"/act", map[string]any{
		"action": "compensate", "amount": 50,
	}, token)
	if over.Code != 422 {
		t.Fatalf("over limit %d %s", over.Code, over.Body.String())
	}
	paid := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices/"+voice["id"].(string)+"/act", map[string]any{
		"action": "compensate", "amount": 10,
	}, token)
	if paid.Code != 200 {
		t.Fatalf("compensate %d %s", paid.Code, paid.Body.String())
	}
	var recorded map[string]any
	decode(t, paid, &recorded)
	if recorded["compensationAmount"] != float64(10) || recorded["compensationCurrency"] != "IQD" || recorded["recoveryStatus"] != "open" {
		t.Fatalf("gesture is a record, not a payment: %s", paid.Body.String())
	}
	assigned := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices/"+voice["id"].(string)+"/act", map[string]any{
		"action": "assign", "assigneeUserId": member["userId"],
	}, token)
	if assigned.Code != 200 {
		t.Fatalf("assign %d %s", assigned.Code, assigned.Body.String())
	}
	var handoff map[string]any
	decode(t, assigned, &handoff)
	if handoff["recoveryStatus"] != "assigned" || handoff["linkedTaskId"] == nil {
		t.Fatalf("assigned=%s", assigned.Body.String())
	}
	closed := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/voices/"+voice["id"].(string)+"/act", map[string]any{
		"action": "close", "note": "tea replaced",
	}, token)
	if closed.Code != 200 {
		t.Fatalf("close %d %s", closed.Code, closed.Body.String())
	}
	var done map[string]any
	decode(t, closed, &done)
	if done["recoveryStatus"] != "closed" {
		t.Fatalf("closed=%s", closed.Body.String())
	}
}

func TestQRUnknownTokenIsNotFound(t *testing.T) {
	h := newHarness(t)
	if rec := get(h.handler, "/v1/public/qr/abcdefghijabcdefghijab", ""); rec.Code != 404 {
		t.Fatalf("unknown %d %s", rec.Code, rec.Body.String())
	}
}

type wrapItems struct {
	Items []map[string]any `json:"items"`
}

func TestQRJSONItemsShape(t *testing.T) {
	raw, _ := json.Marshal(wrapItems{})
	if string(raw) == "" {
		t.Fatal("empty")
	}
}
