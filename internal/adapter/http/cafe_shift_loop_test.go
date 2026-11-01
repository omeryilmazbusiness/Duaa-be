package httpadapter_test

import (
	"net/http/httptest"
	"testing"
	"time"
)

// Three recorded staff, two cafe days, one GM. No invented sales, waste totals, or shifts.
func TestCafeThreeStaffTwoDayManagerLoop(t *testing.T) {
	h := newHarness(t)
	gm := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", completeCafeBody("Duaa Cafe"), gm); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}

	ali := hire(t, h, gm, cafeID, map[string]string{
		"firstName": "Ali", "lastName": "Barista", "staffDepartment": "BAR", "employeeNumber": "6201",
	})
	noor := hire(t, h, gm, cafeID, map[string]string{
		"firstName": "Noor", "lastName": "Kitchen", "staffDepartment": "KITCHEN", "employeeNumber": "6202",
	})
	huda := hire(t, h, gm, cafeID, map[string]string{
		"firstName": "Huda", "lastName": "Floor", "staffDepartment": "SERVICE",
		"email": "huda@cafe.test", "password": "Floor123",
	})
	aliTok := staffToken(t, h, "6201")
	noorTok := staffToken(t, h, "6202")
	hudaTok := emailToken(t, h, "huda@cafe.test", "Floor123")

	day1 := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)

	t1Ali := assign(t, h, gm, cafeID, "open espresso", ali, day1)
	t1Noor := assign(t, h, gm, cafeID, "prep mise", noor, day1)
	t1Huda := assign(t, h, gm, cafeID, "floor briefing", huda, day1)
	if t1Ali == "" || t1Noor == "" || t1Huda == "" {
		t.Fatal("day-1 assignments")
	}
	mustCode(t, patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+t1Ali+"/complete", gm), 200, "complete ali")
	mustCode(t, patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+t1Noor+"/complete", gm), 200, "complete noor")

	mustCode(t, post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "breakage", "title": "portafilter cracked",
	}, aliTok), 201, "ali breakage")
	mustCode(t, post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "request", "title": "need extra milk",
	}, noorTok), 201, "noor request stays off the ariza snapshot")

	codes := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", gm)
	var issued wrapItems
	decode(t, codes, &issued)
	if len(issued.Items) == 0 {
		t.Fatalf("qr=%s", codes.Body.String())
	}
	mustCode(t, post(h.handler, "/v1/public/qr/"+issued.Items[0]["token"].(string), map[string]any{
		"kind": "complaint", "comment": "tea was cold",
	}, ""), 201, "guest recovery")

	mustCode(t, put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/intake", map[string]string{
		"businessDate": "2026-11-01", "shiftName": "Morning",
		"closeNote": "quiet close after briefing",
	}, gm), 200, "day-1 intake")

	pulse1 := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover", gm)
	var p1 map[string]any
	decode(t, pulse1, &p1)
	if p1["persisted"] != false || len(asList(p1["tasks"])) != 1 || len(asList(p1["recoveries"])) != 1 || len(asList(p1["breakdowns"])) != 1 {
		t.Fatalf("day-1 live pulse must be 1/1/1 recorded files: %s", pulse1.Body.String())
	}

	opened1 := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover", map[string]string{
		"businessDate": "2026-11-01", "fromShift": "Morning", "toShift": "Morning",
		"note": "keys with Huda",
	}, gm)
	mustCode(t, opened1, 201, "open day-1")
	var file1 map[string]any
	decode(t, opened1, &file1)
	mustCode(t, patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+t1Huda+"/complete", gm), 200, "complete huda after open")
	frozen := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover", gm)
	var still map[string]any
	decode(t, frozen, &still)
	if still["id"] != file1["id"] || len(asList(still["tasks"])) != 1 {
		t.Fatalf("day-1 snapshot must freeze: %s", frozen.Body.String())
	}
	mustCode(t, post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover/"+file1["id"].(string)+"/accept", map[string]string{}, hudaTok), 200, "huda accepts")

	assign(t, h, gm, cafeID, "restock milk", ali, day2)
	assign(t, h, gm, cafeID, "dough proof", noor, day2)
	t2Huda := assign(t, h, gm, cafeID, "open garden", huda, day2)
	mustCode(t, post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "breakage", "title": "garden lamp out",
	}, hudaTok), 201, "huda breakage")
	mustCode(t, put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/intake", map[string]string{
		"businessDate": "2026-11-02", "shiftName": "Morning",
		"closeNote": "second day close", "incidentNote": "lamp reported",
	}, gm), 200, "day-2 intake")

	pulse2 := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover", gm)
	var p2 map[string]any
	decode(t, pulse2, &p2)
	if p2["persisted"] != false || len(asList(p2["tasks"])) != 3 {
		t.Fatalf("day-2 live work is the three new tasks: %s", pulse2.Body.String())
	}
	if len(asList(p2["breakdowns"])) != 2 || len(asList(p2["recoveries"])) != 1 {
		t.Fatalf("day-2 pulse must keep recorded files only: %s", pulse2.Body.String())
	}

	opened2 := post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover", map[string]string{
		"businessDate": "2026-11-02", "fromShift": "Morning", "note": "garden lamp still open",
	}, gm)
	mustCode(t, opened2, 201, "open day-2")
	var file2 map[string]any
	decode(t, opened2, &file2)
	if file2["id"] == file1["id"] {
		t.Fatal("day-2 is a new file")
	}
	mustCode(t, post(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handover/"+file2["id"].(string)+"/accept", map[string]string{}, gm), 200, "gm accepts day-2")

	intakes := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/intakes", gm)
	var days wrapItems
	decode(t, intakes, &days)
	if len(days.Items) != 2 {
		t.Fatalf("two recorded closes: %s", intakes.Body.String())
	}
	hands := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handovers", gm)
	var files wrapItems
	decode(t, hands, &files)
	if len(files.Items) != 2 || files.Items[0]["status"] != "accepted" || files.Items[1]["status"] != "accepted" {
		t.Fatalf("two accepted handovers: %s", hands.Body.String())
	}
	if t2Huda == "" {
		t.Fatal("day-2 huda task")
	}
}

func hire(t *testing.T, h harness, token, cafeID string, body map[string]string) string {
	t.Helper()
	rec := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", body, token)
	if rec.Code != 201 {
		t.Fatalf("hire %s %d %s", body["firstName"], rec.Code, rec.Body.String())
	}
	var member map[string]any
	decode(t, rec, &member)
	id, _ := member["userId"].(string)
	if id == "" {
		t.Fatalf("hire userId=%s", rec.Body.String())
	}
	return id
}

func assign(t *testing.T, h harness, token, cafeID, title, assignee string, start time.Time) string {
	t.Helper()
	rec := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": title, "assigneeUserId": assignee,
		"scheduledStartAt": start.Format(time.RFC3339),
		"scheduledEndAt":   start.Add(2 * time.Hour).Format(time.RFC3339),
	}, token)
	if rec.Code != 201 {
		t.Fatalf("assign %s %d %s", title, rec.Code, rec.Body.String())
	}
	var view map[string]any
	decode(t, rec, &view)
	id, _ := view["id"].(string)
	return id
}

func staffToken(t *testing.T, h harness, number string) string {
	t.Helper()
	rec := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": number,
	}, "")
	if rec.Code != 200 {
		t.Fatalf("staff %s %d %s", number, rec.Code, rec.Body.String())
	}
	var session wrapLogin
	decode(t, rec, &session)
	return session.Token
}

func emailToken(t *testing.T, h harness, email, password string) string {
	t.Helper()
	rec := post(h.handler, "/v1/auth/login", map[string]string{"email": email, "password": password}, "")
	if rec.Code != 200 {
		t.Fatalf("login %s %d %s", email, rec.Code, rec.Body.String())
	}
	var session wrapLogin
	decode(t, rec, &session)
	return session.Token
}

func mustCode(t *testing.T, rec *httptest.ResponseRecorder, want int, label string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s %d %s", label, rec.Code, rec.Body.String())
	}
}
