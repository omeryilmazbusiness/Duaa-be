package httpadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/adapter/assistantctx"
	httpadapter "github.com/dua-os/dua-os-be/internal/adapter/http"
	"github.com/dua-os/dua-os-be/internal/adapter/memory"
	"github.com/dua-os/dua-os-be/internal/app/actor"
	appapproval "github.com/dua-os/dua-os-be/internal/app/approval"
	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	appauth "github.com/dua-os/dua-os-be/internal/app/auth"
	appbrief "github.com/dua-os/dua-os-be/internal/app/brief"
	appcash "github.com/dua-os/dua-os-be/internal/app/cash"
	appdashboard "github.com/dua-os/dua-os-be/internal/app/dashboard"
	appdecision "github.com/dua-os/dua-os-be/internal/app/decision"
	appreport "github.com/dua-os/dua-os-be/internal/app/report"
	appstaff "github.com/dua-os/dua-os-be/internal/app/staff"
	apptask "github.com/dua-os/dua-os-be/internal/app/task"
	"github.com/dua-os/dua-os-be/internal/bootstrap"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/supply"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/platform/authn"
	"github.com/dua-os/dua-os-be/internal/platform/config"
	"github.com/dua-os/dua-os-be/internal/platform/secretbox"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type harness struct {
	handler http.Handler
	seed    bootstrap.SeedResult
	store   *memory.Store
	vault   *secretbox.Box
	llm     *fakeLLM
}

func newHarness(t *testing.T) harness {
	t.Helper()
	store := memory.New()
	clk := clock.System{}
	seed, err := bootstrap.SeedCafe(t.Context(), store, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := authn.NewJWT("dev-only-change-me-to-at-least-32-chars", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	reg := tenancy.NewRegistry(cafe.Catalog{}, hotel.Catalog{}, supply.Catalog{})
	passwords := authn.Bcrypt{}
	tasks := apptask.New(store, store, clk)
	briefs := appbrief.New(store, clk, appbrief.TaskSignal{Tasks: store, Members: store}, appbrief.CashSignal{Closes: store})
	reports := appreport.New(store, store, store, harnessAssigner{tasks: tasks}, clk)
	dash := appdashboard.New(store, store, store, store, store, store, clk)
	vault, err := secretbox.New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeLLM{}
	api := httpadapter.API{
		Auth:      appauth.New(store, store, store, reg, tokens, passwords, clk),
		Staff:     appstaff.New(store, store, passwords, clk),
		Tasks:     tasks,
		Brief:     briefs,
		Decisions: appdecision.New(store, store, harnessAssigner{tasks: tasks}, clk),
		Reports:   reports,
		Cash:      appcash.New(store, clk),
		Approvals: appapproval.New(store, clk),
		Dashboard: dash,
		Assistant: appassistant.New(store, store, vault, fake, assistantctx.New(dash, briefs, tasks, reports), fake, clk),
		Actors:    actor.NewLoader(store, reg),
		JWT:       tokens,
	}
	cfg := config.Config{Version: "test", Env: "test", CORSOrigins: []string{"*"}}
	return harness{handler: httpadapter.NewRouter(cfg, api), seed: seed, store: store, vault: vault, llm: fake}
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
}

func post(h http.Handler, path string, body any, token string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func patch(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(h http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	h := newHarness(t)
	if rec := get(h.handler, "/health", ""); rec.Code != 200 {
		t.Fatalf("health %d", rec.Code)
	}
}

func TestCafeGMManagerEmployeeLoop(t *testing.T) {
	h := newHarness(t)
	cafeID := h.seed.CafeID.String()

	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	if login.Code != 200 {
		t.Fatalf("login %d %s", login.Code, login.Body.String())
	}
	var session wrapLogin
	decode(t, login, &session)
	if session.NeedsPick {
		t.Fatal("cafe-first seed has one workspace; GM should enter cafe directly")
	}
	if session.Session.Scope != "general_manager" || session.Session.Workspace.Kind != "cafe" {
		t.Fatalf("scope=%s kind=%s", session.Session.Scope, session.Session.Workspace.Kind)
	}
	if contains(session.Session.Tabs, "reservations") {
		t.Fatal("cafe GM must not see hotel reservations")
	}
	if !contains(session.Session.Tabs, "feedback") {
		t.Fatal("cafe GM must see feedback")
	}
	gmToken := session.Token

	createdMgr := post(h.handler, "/v1/workspaces/"+cafeID+"/staff/department-managers", map[string]string{
		"firstName": "Layla", "lastName": "Kitchen",
		"email": "layla@cafe.test", "password": "Kitchen123",
		"staffDepartment": "KITCHEN",
	}, gmToken)
	if createdMgr.Code != 201 {
		t.Fatalf("dept mgr %d %s", createdMgr.Code, createdMgr.Body.String())
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Barista",
		"staffDepartment": "BAR", "employeeNumber": "1042",
	}, gmToken)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	var emp map[string]any
	decode(t, createdEmp, &emp)

	mgrLogin := post(h.handler, "/v1/auth/login", map[string]string{
		"email": "layla@cafe.test", "password": "Kitchen123",
	}, "")
	var mgrSession wrapLogin
	decode(t, mgrLogin, &mgrSession)
	if mgrSession.Session.Scope != "department_manager" {
		t.Fatalf("mgr scope=%s", mgrSession.Session.Scope)
	}

	start := time.Now().UTC()
	end := start.Add(2 * time.Hour)
	taskRec := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": "Open espresso machine", "assigneeUserId": emp["userId"].(string),
		"scheduledStartAt": start.Format(time.RFC3339), "scheduledEndAt": end.Format(time.RFC3339),
	}, mgrSession.Token)
	if taskRec.Code != 403 {
		t.Fatalf("kitchen manager assigning BAR staff must be forbidden, got %d %s", taskRec.Code, taskRec.Body.String())
	}

	gmTask := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": "Open espresso machine", "assigneeUserId": emp["userId"].(string),
		"scheduledStartAt": start.Format(time.RFC3339), "scheduledEndAt": end.Format(time.RFC3339),
	}, gmToken)
	if gmTask.Code != 201 {
		t.Fatalf("gm task %d %s", gmTask.Code, gmTask.Body.String())
	}
	var task map[string]any
	decode(t, gmTask, &task)

	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1042",
	}, "")
	if staffLogin.Code != 200 {
		t.Fatalf("staff login %d %s", staffLogin.Code, staffLogin.Body.String())
	}
	var staffSession wrapLogin
	decode(t, staffLogin, &staffSession)
	if staffSession.Session.Surface != "staff_portal" {
		t.Fatalf("surface=%s", staffSession.Session.Surface)
	}

	complete := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+cafeID+"/staff-portal/tasks/"+task["id"].(string)+"/complete", nil)
	complete.Header.Set("Authorization", "Bearer "+staffSession.Token)
	completeRec := httptest.NewRecorder()
	h.handler.ServeHTTP(completeRec, complete)
	if completeRec.Code != 200 {
		t.Fatalf("complete %d %s", completeRec.Code, completeRec.Body.String())
	}

	foreign := get(h.handler, "/v1/workspaces/"+id.New().String()+"/staff", gmToken)
	if foreign.Code != 403 {
		t.Fatalf("cafe token must not open another workspace, got %d %s", foreign.Code, foreign.Body.String())
	}

	gmLate := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": "Restock milk", "assigneeUserId": emp["userId"].(string),
		"scheduledStartAt": start.Add(-3 * time.Hour).Format(time.RFC3339),
		"scheduledEndAt":   start.Add(-time.Hour).Format(time.RFC3339),
	}, gmToken)
	if gmLate.Code != 201 {
		t.Fatalf("gm overdue task %d %s", gmLate.Code, gmLate.Body.String())
	}
	var late map[string]any
	decode(t, gmLate, &late)
	gmDone := patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+late["id"].(string)+"/complete", gmToken)
	if gmDone.Code != 200 {
		t.Fatalf("gm complete overdue %d %s", gmDone.Code, gmDone.Body.String())
	}
	staffDone := patch(h.handler, "/v1/workspaces/"+cafeID+"/tasks/"+late["id"].(string)+"/complete", staffSession.Token)
	if staffDone.Code != 403 {
		t.Fatalf("staff must not use manager complete, got %d %s", staffDone.Code, staffDone.Body.String())
	}
}

func TestEmployeeCannotListStaff(t *testing.T) {
	h := newHarness(t)
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	var gm wrapLogin
	decode(t, login, &gm)
	created := post(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff", map[string]string{
		"firstName": "Noor", "lastName": "Clean", "staffDepartment": "CLEANING", "employeeNumber": "2001",
	}, gm.Token)
	if created.Code != 201 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "2001",
	}, "")
	var emp wrapLogin
	decode(t, staffLogin, &emp)
	list := get(h.handler, "/v1/workspaces/"+h.seed.CafeID.String()+"/staff", emp.Token)
	if list.Code != 403 {
		t.Fatalf("employee listed staff: %d %s", list.Code, list.Body.String())
	}
}

type wrapLogin struct {
	Token     string `json:"token"`
	NeedsPick bool   `json:"needsWorkspacePick"`
	Session   struct {
		Scope     string   `json:"scope"`
		Surface   string   `json:"surface"`
		Tabs      []string `json:"tabs"`
		Workspace struct {
			Kind string `json:"kind"`
		} `json:"workspace"`
	} `json:"session"`
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

type harnessAssigner struct{ tasks *apptask.Service }

func (a harnessAssigner) Assign(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (id.ID, error) {
	view, err := a.tasks.Create(ctx, ac, title, description, assigneeUserID, start, end)
	if err != nil {
		return id.Zero(), err
	}
	return id.Parse(view.ID)
}

func TestGMTodayBriefUsesLiveTasksAndHonestSources(t *testing.T) {
	h := newHarness(t)
	cafeID := h.seed.CafeID.String()
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	var gm wrapLogin
	decode(t, login, &gm)

	emptyBrief := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	if emptyBrief.Code != 200 {
		t.Fatalf("brief %d %s", emptyBrief.Code, emptyBrief.Body.String())
	}
	var empty map[string]any
	decode(t, emptyBrief, &empty)
	if empty["isClear"] != true {
		t.Fatalf("seed cafe has no tasks, brief must be clear: %s", emptyBrief.Body.String())
	}
	cards, _ := empty["cards"].([]any)
	if len(cards) != 0 {
		t.Fatalf("clear morning has no cards: %v", cards)
	}
	sources, _ := empty["sources"].([]any)
	foundVoice, foundCash := false, false
	for _, raw := range sources {
		src, _ := raw.(map[string]any)
		if src["kind"] == "customer_voice" {
			foundVoice = true
			if src["status"] != "unavailable" {
				t.Fatalf("reviews must be unavailable, not a fake card: %v", src)
			}
		}
		if src["kind"] == "cash_variance" {
			foundCash = true
			if src["status"] != "unavailable" {
				t.Fatalf("unclosed till must be unavailable, not a fake card: %v", src)
			}
		}
	}
	if !foundVoice {
		t.Fatal("cafe catalogues customer voice as an honest missing source")
	}
	if !foundCash {
		t.Fatal("cafe catalogues cash as an honest missing source until the till is closed")
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Barista",
		"staffDepartment": "BAR", "employeeNumber": "1043",
	}, gm.Token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	var emp map[string]any
	decode(t, createdEmp, &emp)

	start := time.Now().UTC().Add(-3 * time.Hour)
	end := time.Now().UTC().Add(-time.Hour)
	taskRec := post(h.handler, "/v1/workspaces/"+cafeID+"/tasks", map[string]string{
		"title": "Open espresso machine", "assigneeUserId": emp["userId"].(string),
		"scheduledStartAt": start.Format(time.RFC3339), "scheduledEndAt": end.Format(time.RFC3339),
	}, gm.Token)
	if taskRec.Code != 201 {
		t.Fatalf("task %d %s", taskRec.Code, taskRec.Body.String())
	}

	live := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	if live.Code != 200 {
		t.Fatalf("live brief %d %s", live.Code, live.Body.String())
	}
	var brief map[string]any
	decode(t, live, &brief)
	if brief["isClear"] == true {
		t.Fatalf("overdue task must raise a card: %s", live.Body.String())
	}
	liveCards, _ := brief["cards"].([]any)
	if len(liveCards) != 1 {
		t.Fatalf("expected one late-staff card, got %s", live.Body.String())
	}
	card, _ := liveCards[0].(map[string]any)
	if card["kind"] != "late_staff" || card["count"] != float64(1) {
		t.Fatalf("card=%v", card)
	}

	ack := post(h.handler, "/v1/workspaces/"+cafeID+"/executive/act", map[string]any{
		"sourceKey": card["sourceKey"], "fingerprint": card["fingerprint"],
		"kind": card["kind"], "titleKey": card["titleKey"], "detail": card["detail"],
		"severity": card["severity"], "targetTab": card["targetTab"], "action": "acknowledge",
	}, gm.Token)
	if ack.Code != 200 {
		t.Fatalf("ack %d %s", ack.Code, ack.Body.String())
	}

	assign := post(h.handler, "/v1/workspaces/"+cafeID+"/executive/act", map[string]any{
		"sourceKey": card["sourceKey"], "fingerprint": card["fingerprint"],
		"kind": card["kind"], "titleKey": card["titleKey"], "detail": card["detail"],
		"severity": card["severity"], "targetTab": card["targetTab"],
		"action": "assign", "assigneeUserId": emp["userId"].(string),
		"title": "Chase late espresso open",
	}, gm.Token)
	if assign.Code != 200 {
		t.Fatalf("assign %d %s", assign.Code, assign.Body.String())
	}
	var assigned map[string]any
	decode(t, assign, &assigned)
	if assigned["status"] != "assigned" || assigned["linkedTaskId"] == nil {
		t.Fatalf("assigned=%v", assigned)
	}

	still := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	var stillBrief map[string]any
	decode(t, still, &stillBrief)
	stillCards, _ := stillBrief["cards"].([]any)
	if len(stillCards) != 1 {
		t.Fatalf("assigned work stays on Today until done: %s", still.Body.String())
	}
	stillCard, _ := stillCards[0].(map[string]any)
	dec, _ := stillCard["decision"].(map[string]any)
	if dec["status"] != "assigned" {
		t.Fatalf("decision=%v", dec)
	}

	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1043",
	}, "")
	var staff wrapLogin
	decode(t, staffLogin, &staff)
	complete := httptest.NewRequest(http.MethodPatch, "/v1/workspaces/"+cafeID+"/staff-portal/tasks/"+assigned["linkedTaskId"].(string)+"/complete", nil)
	complete.Header.Set("Authorization", "Bearer "+staff.Token)
	completeRec := httptest.NewRecorder()
	h.handler.ServeHTTP(completeRec, complete)
	if completeRec.Code != 200 {
		t.Fatalf("complete follow-up %d %s", completeRec.Code, completeRec.Body.String())
	}

	reopened := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	var reopenedBrief map[string]any
	decode(t, reopened, &reopenedBrief)
	reopenedCards, _ := reopenedBrief["cards"].([]any)
	if len(reopenedCards) != 1 {
		t.Fatalf("original overdue work must return after follow-up is done: %s", reopened.Body.String())
	}

	resolve := post(h.handler, "/v1/workspaces/"+cafeID+"/executive/act", map[string]any{
		"sourceKey": card["sourceKey"], "fingerprint": card["fingerprint"],
		"kind": card["kind"], "action": "resolve",
	}, gm.Token)
	if resolve.Code != 200 {
		t.Fatalf("resolve %d %s", resolve.Code, resolve.Body.String())
	}

	after := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	var afterBrief map[string]any
	decode(t, after, &afterBrief)
	afterCards, _ := afterBrief["cards"].([]any)
	if len(afterCards) != 0 {
		t.Fatalf("GM done snoozes the same fingerprint until tomorrow: %s", after.Body.String())
	}
}

func TestEmployeeReportLandsInGMFeedback(t *testing.T) {
	h := newHarness(t)
	cafeID := h.seed.CafeID.String()
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	var gm wrapLogin
	decode(t, login, &gm)

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Barista",
		"staffDepartment": "BAR", "employeeNumber": "1044",
	}, gm.Token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	var emp map[string]any
	decode(t, createdEmp, &emp)

	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1044",
	}, "")
	var staff wrapLogin
	decode(t, staffLogin, &staff)

	blank := post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "breakage", "title": "  ",
	}, staff.Token)
	if blank.Code != 422 {
		t.Fatalf("blank title must be invalid, got %d %s", blank.Code, blank.Body.String())
	}

	created := post(h.handler, "/v1/workspaces/"+cafeID+"/reports", map[string]string{
		"kind": "breakage", "title": "Espresso machine jammed", "detail": "won't grind",
	}, staff.Token)
	if created.Code != 201 {
		t.Fatalf("submit %d %s", created.Code, created.Body.String())
	}
	var report map[string]any
	decode(t, created, &report)
	if report["status"] != "open" || report["kind"] != "breakage" {
		t.Fatalf("report=%v", report)
	}

	staffList := get(h.handler, "/v1/workspaces/"+cafeID+"/reports", staff.Token)
	if staffList.Code != 200 {
		t.Fatalf("staff list %d %s", staffList.Code, staffList.Body.String())
	}
	var own []map[string]any
	decode(t, staffList, &own)
	if len(own) != 1 {
		t.Fatalf("staff must see own report, got %s", staffList.Body.String())
	}

	actForbidden := post(h.handler, "/v1/workspaces/"+cafeID+"/reports/"+report["id"].(string)+"/act", map[string]string{
		"action": "resolve",
	}, staff.Token)
	if actForbidden.Code != 403 {
		t.Fatalf("staff must not act, got %d %s", actForbidden.Code, actForbidden.Body.String())
	}

	inbox := get(h.handler, "/v1/workspaces/"+cafeID+"/reports", gm.Token)
	if inbox.Code != 200 {
		t.Fatalf("gm list %d %s", inbox.Code, inbox.Body.String())
	}
	var listed []map[string]any
	decode(t, inbox, &listed)
	if len(listed) != 1 || listed[0]["title"] != "Espresso machine jammed" {
		t.Fatalf("gm inbox=%s", inbox.Body.String())
	}

	assigned := post(h.handler, "/v1/workspaces/"+cafeID+"/reports/"+report["id"].(string)+"/act", map[string]any{
		"action": "assign", "assigneeUserId": emp["userId"],
	}, gm.Token)
	if assigned.Code != 200 {
		t.Fatalf("assign %d %s", assigned.Code, assigned.Body.String())
	}
	var acted map[string]any
	decode(t, assigned, &acted)
	if acted["status"] != "assigned" || acted["linkedTaskId"] == nil {
		t.Fatalf("assigned=%v", acted)
	}

	portal := get(h.handler, "/v1/workspaces/"+cafeID+"/staff-portal/tasks", staff.Token)
	if portal.Code != 200 {
		t.Fatalf("portal %d %s", portal.Code, portal.Body.String())
	}
	var tasks []map[string]any
	decode(t, portal, &tasks)
	found := false
	for _, task := range tasks {
		if task["id"] == acted["linkedTaskId"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("assigned follow-up must land on staff portal: %s", portal.Body.String())
	}

	resolved := post(h.handler, "/v1/workspaces/"+cafeID+"/reports/"+report["id"].(string)+"/act", map[string]string{
		"action": "resolve",
	}, gm.Token)
	if resolved.Code != 200 {
		t.Fatalf("resolve %d %s", resolved.Code, resolved.Body.String())
	}
	var closed map[string]any
	decode(t, resolved, &closed)
	if closed["status"] != "resolved" {
		t.Fatalf("closed=%v", closed)
	}
}

func TestGMDashboardCashAndApprovals(t *testing.T) {
	h := newHarness(t)
	cafeID := h.seed.CafeID.String()
	login := post(h.handler, "/v1/auth/login", map[string]string{
		"email": h.seed.GMEmail, "password": h.seed.GMPassword,
	}, "")
	var gm wrapLogin
	decode(t, login, &gm)

	dash := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/dashboard", gm.Token)
	if dash.Code != 200 {
		t.Fatalf("dashboard %d %s", dash.Code, dash.Body.String())
	}
	var view map[string]any
	decode(t, dash, &view)
	tiles, _ := view["tiles"].([]any)
	foundSales, foundCash, salesLive := false, false, false
	for _, raw := range tiles {
		tile, _ := raw.(map[string]any)
		if tile["module"] == "sales" {
			foundSales = true
			if tile["status"] == "live" {
				salesLive = true
			}
			if tile["value"] != float64(0) && tile["status"] == "unavailable" {
				// unavailable tiles keep value 0 internally; they must not be presented as a sale
			}
		}
		if tile["module"] == "cash" {
			foundCash = true
			if tile["status"] != "unavailable" {
				t.Fatalf("unclosed till must be unavailable: %v", tile)
			}
		}
	}
	if !foundSales || salesLive {
		t.Fatal("POS sales must stay an honest missing tile, never a fake live zero")
	}
	if !foundCash {
		t.Fatal("cafe dashboard catalogues cash")
	}

	staffLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "does-not-exist",
	}, "")
	if staffLogin.Code == 200 {
		t.Fatal("unknown employee must not login")
	}

	createdEmp := post(h.handler, "/v1/workspaces/"+cafeID+"/staff", map[string]string{
		"firstName": "Ali", "lastName": "Barista", "staffDepartment": "BAR", "employeeNumber": "1099",
	}, gm.Token)
	if createdEmp.Code != 201 {
		t.Fatalf("employee %d %s", createdEmp.Code, createdEmp.Body.String())
	}
	empLogin := post(h.handler, "/v1/auth/staff-login", map[string]string{
		"workspaceSlug": "duaa-cafe", "employeeNumber": "1099",
	}, "")
	var emp wrapLogin
	decode(t, empLogin, &emp)
	forbidden := post(h.handler, "/v1/workspaces/"+cafeID+"/cash/close", map[string]any{
		"countedAmount": 1000,
	}, emp.Token)
	if forbidden.Code != 403 {
		t.Fatalf("staff must not close till, got %d %s", forbidden.Code, forbidden.Body.String())
	}

	expected := int64(100000)
	closed := post(h.handler, "/v1/workspaces/"+cafeID+"/cash/close", map[string]any{
		"countedAmount": 98000, "expectedAmount": expected, "note": "short drawer",
	}, gm.Token)
	if closed.Code != 200 {
		t.Fatalf("close %d %s", closed.Code, closed.Body.String())
	}

	brief := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/brief", gm.Token)
	var briefView map[string]any
	decode(t, brief, &briefView)
	cards, _ := briefView["cards"].([]any)
	foundVariance := false
	for _, raw := range cards {
		card, _ := raw.(map[string]any)
		if card["kind"] == "cash_variance" {
			foundVariance = true
		}
	}
	if !foundVariance {
		t.Fatalf("closed till with variance must raise a Today card: %s", brief.Body.String())
	}

	pack := post(h.handler, "/v1/workspaces/"+cafeID+"/approvals", map[string]any{
		"title": "Milk invoice", "supplier": "Al-Nahrain", "amount": 250000, "evidence": "photo",
	}, gm.Token)
	if pack.Code != 201 {
		t.Fatalf("approval %d %s", pack.Code, pack.Body.String())
	}
	var created map[string]any
	decode(t, pack, &created)

	approved := post(h.handler, "/v1/workspaces/"+cafeID+"/approvals/"+created["id"].(string)+"/act", map[string]any{
		"action": "approve",
	}, gm.Token)
	if approved.Code != 200 {
		t.Fatalf("approve %d %s", approved.Code, approved.Body.String())
	}
	revised := post(h.handler, "/v1/workspaces/"+cafeID+"/approvals/"+created["id"].(string)+"/act", map[string]any{
		"amount": 260000,
	}, gm.Token)
	if revised.Code != 200 {
		t.Fatalf("revise %d %s", revised.Code, revised.Body.String())
	}
	var after map[string]any
	decode(t, revised, &after)
	if after["status"] != "open" || after["amount"] != float64(260000) {
		t.Fatalf("amount change must drop approval: %v", after)
	}

	live := get(h.handler, "/v1/workspaces/"+cafeID+"/executive/dashboard", gm.Token)
	var liveView map[string]any
	decode(t, live, &liveView)
	cash, _ := liveView["cash"].(map[string]any)
	if cash["status"] != "live" || cash["variance"] != float64(-2000) {
		t.Fatalf("dashboard cash=%v", cash)
	}
	if pulse, _ := liveView["pulse"].(map[string]any); pulse["employees"] != float64(1) {
		t.Fatalf("pulse=%v", liveView["pulse"])
	}
}
