package httpadapter_test

import (
	"testing"
	"time"
)

func TestCafeHandoverEmptySnapshotThenAccept(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	cafeID := h.seed.CafeID.String()
	path := "/v1/workspaces/" + cafeID + "/cafe/handover"

	empty := get(h.handler, path, token)
	if empty.Code != 200 {
		t.Fatalf("empty %d %s", empty.Code, empty.Body.String())
	}
	var preview map[string]any
	decode(t, empty, &preview)
	if preview["persisted"] != false {
		t.Fatalf("must not invent a file: %s", empty.Body.String())
	}
	if len(asList(preview["tasks"]))+len(asList(preview["recoveries"]))+len(asList(preview["breakdowns"])) != 0 {
		t.Fatalf("empty pulse invented work: %s", empty.Body.String())
	}
	if rec := post(h.handler, path, map[string]string{"fromShift": "Night"}, token); rec.Code != 422 {
		t.Fatalf("invented shift %d %s", rec.Code, rec.Body.String())
	}

	if rec := put(h.handler, "/v1/workspaces/"+cafeID+"/cafe/definition", completeCafeBody("Duaa Cafe"), token); rec.Code != 200 {
		t.Fatalf("definition %d %s", rec.Code, rec.Body.String())
	}
	emp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Bar", "staffDepartment": "BAR", "employeeNumber": "5101",
	}, token)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	var member map[string]any
	decode(t, emp, &member)
	start := time.Now().UTC()
	taskRec := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": "restock milk", "assigneeUserId": member["userId"].(string),
		"scheduledStartAt": start.Format(time.RFC3339), "scheduledEndAt": start.Add(2 * time.Hour).Format(time.RFC3339),
	}, token)
	if taskRec.Code != 201 {
		t.Fatalf("task %d %s", taskRec.Code, taskRec.Body.String())
	}
	var task map[string]any
	decode(t, taskRec, &task)

	staff := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "5101",
	}, "")
	var session wrapLogin
	decode(t, staff, &session)
	rep := post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "breakage", "title": "espresso hopper cracked",
	}, session.Token)
	if rep.Code != 201 {
		t.Fatalf("report %d %s", rep.Code, rep.Body.String())
	}

	codes := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/qr", token)
	var issued wrapItems
	decode(t, codes, &issued)
	if len(issued.Items) == 0 {
		t.Fatalf("qr=%s", codes.Body.String())
	}
	voice := post(h.handler, "/v1/public/qr/"+issued.Items[0]["token"].(string), map[string]any{"kind": "complaint", "comment": "tea was cold"}, "")
	if voice.Code != 201 {
		t.Fatalf("voice %d %s", voice.Code, voice.Body.String())
	}

	live := get(h.handler, path, token)
	var pulse map[string]any
	decode(t, live, &pulse)
	if pulse["persisted"] != false || len(asList(pulse["tasks"])) != 1 || len(asList(pulse["recoveries"])) != 1 || len(asList(pulse["breakdowns"])) != 1 {
		t.Fatalf("live pulse=%s", live.Body.String())
	}

	opened := post(h.handler, path, map[string]string{"fromShift": "Morning", "note": "keys on bar"}, token)
	if opened.Code != 201 {
		t.Fatalf("open %d %s", opened.Code, opened.Body.String())
	}
	var file map[string]any
	decode(t, opened, &file)
	if file["persisted"] != true || file["status"] != "open" || file["fromShift"] != "Morning" || file["note"] != "keys on bar" {
		t.Fatalf("opened=%s", opened.Body.String())
	}
	if rec := post(h.handler, path, map[string]string{}, token); rec.Code != 409 {
		t.Fatalf("second open %d %s", rec.Code, rec.Body.String())
	}

	done := patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+task["id"].(string)+"/complete", token)
	if done.Code != 200 {
		t.Fatalf("complete %d %s", done.Code, done.Body.String())
	}
	frozen := get(h.handler, path, token)
	var still map[string]any
	decode(t, frozen, &still)
	if still["id"] != file["id"] || len(asList(still["tasks"])) != 1 {
		t.Fatalf("snapshot must freeze: %s", frozen.Body.String())
	}

	accepted := post(h.handler, path+"/"+file["id"].(string)+"/accept", map[string]string{}, token)
	if accepted.Code != 200 {
		t.Fatalf("accept %d %s", accepted.Code, accepted.Body.String())
	}
	var closed map[string]any
	decode(t, accepted, &closed)
	if closed["status"] != "accepted" {
		t.Fatalf("accepted=%s", accepted.Body.String())
	}
	after := get(h.handler, path, token)
	var next map[string]any
	decode(t, after, &next)
	if next["persisted"] != false || len(asList(next["tasks"])) != 0 {
		t.Fatalf("after accept live pulse: %s", after.Body.String())
	}
	list := get(h.handler, "/v1/workspaces/"+cafeID+"/cafe/handovers", token)
	var items wrapItems
	decode(t, list, &items)
	if len(items.Items) != 1 {
		t.Fatalf("list=%s", list.Body.String())
	}
}

func TestCafeHandoverHotelAndStaffRejected(t *testing.T) {
	h := newHarness(t)
	token := gmToken(t, h)
	emp := post(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Bar", "staffDepartment": "BAR", "employeeNumber": "5102",
	}, token)
	if emp.Code != 201 {
		t.Fatalf("emp %d %s", emp.Code, emp.Body.String())
	}
	staff := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "5102",
	}, "")
	var session wrapLogin
	decode(t, staff, &session)
	if rec := get(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/cafe/handover", session.Token); rec.Code != 403 {
		t.Fatalf("staff %d %s", rec.Code, rec.Body.String())
	}
}

func asList(v any) []any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	return list
}
