package inbox_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	appinbox "github.com/dua-os/dua-os-be/internal/app/inbox"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type stubLLM struct{}

func (stubLLM) Complete(_ context.Context, req appinbox.CompletionRequest) (appinbox.CompletionResult, error) {
	if strings.Contains(req.System, "ultra-professional") && strings.Contains(req.System, "translated") {
		return appinbox.CompletionResult{Text: `{"translated":"Dear guest, we confirm your request."}`}, nil
	}
	return appinbox.CompletionResult{Text: `{"category":"guest","summary":"Guest asks for late checkout.","bodyAr":"يطلب الضيف مغادرة متأخرة.","detectedLanguage":"en"}`}, nil
}

type memRepo struct {
	conn inbox.Connection
	have bool
	msgs map[id.ID]inbox.Message
}

func (m *memRepo) GetConnection(_ context.Context, workspaceID id.ID) (inbox.Connection, error) {
	if !m.have || m.conn.WorkspaceID != workspaceID {
		return inbox.Connection{}, inbox.ConnectionNotFoundError()
	}
	return m.conn, nil
}
func (m *memRepo) SaveConnection(_ context.Context, c inbox.Connection) error {
	m.conn, m.have = c, true
	return nil
}
func (m *memRepo) DeleteConnection(_ context.Context, _ id.ID) error {
	m.have = false
	m.msgs = map[id.ID]inbox.Message{}
	return nil
}
func (m *memRepo) UpsertMessage(_ context.Context, msg inbox.Message) error {
	if m.msgs == nil {
		m.msgs = map[id.ID]inbox.Message{}
	}
	for k, existing := range m.msgs {
		if existing.WorkspaceID == msg.WorkspaceID && existing.ExternalID == msg.ExternalID {
			msg.ID = existing.ID
			m.msgs[k] = msg
			return nil
		}
	}
	m.msgs[msg.ID] = msg
	return nil
}
func (m *memRepo) GetMessage(_ context.Context, workspaceID, messageID id.ID) (inbox.Message, error) {
	msg, ok := m.msgs[messageID]
	if !ok || msg.WorkspaceID != workspaceID {
		return inbox.Message{}, inbox.MessageNotFoundError()
	}
	return msg, nil
}
func (m *memRepo) ListMessages(_ context.Context, workspaceID id.ID, _ int) ([]inbox.Message, error) {
	out := make([]inbox.Message, 0)
	for _, msg := range m.msgs {
		if msg.WorkspaceID == workspaceID {
			out = append(out, msg)
		}
	}
	return out, nil
}
func (m *memRepo) CountUnread(_ context.Context, workspaceID id.ID) (int, error) {
	n := 0
	for _, msg := range m.msgs {
		if msg.WorkspaceID == workspaceID && !msg.IsRead && msg.Folder == inbox.FolderInbox {
			n++
		}
	}
	return n, nil
}
func (m *memRepo) DeleteMessages(_ context.Context, workspaceID id.ID) error {
	for k, msg := range m.msgs {
		if msg.WorkspaceID == workspaceID {
			delete(m.msgs, k)
		}
	}
	return nil
}

type passCipher struct{}

func (passCipher) Seal(plaintext, _ string) (string, error) { return "sealed:" + plaintext, nil }
func (passCipher) Open(sealed, _ string) (string, error) {
	return strings.TrimPrefix(sealed, "sealed:"), nil
}

type settingsRepo struct {
	s assistant.Settings
}

func (r settingsRepo) GetAssistantSettings(_ context.Context, workspaceID id.ID) (assistant.Settings, error) {
	if r.s.WorkspaceID != workspaceID {
		return assistant.Settings{}, inbox.ConnectionNotFoundError()
	}
	return r.s, nil
}

type usageRepo struct{ used int }

func (u *usageRepo) ReserveAIUsage(_ context.Context, _ id.ID, _ string, limit int) (int, bool, error) {
	if u.used >= limit {
		return u.used, false, nil
	}
	u.used++
	return u.used, true, nil
}
func (u *usageRepo) ReleaseAIUsage(_ context.Context, _ id.ID, _ string) error {
	if u.used > 0 {
		u.used--
	}
	return nil
}

type demoBox struct{}

func (demoBox) Connect(_ context.Context, workspaceID id.ID) (inbox.Connection, []inbox.RemoteMessage, error) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	return inbox.Connection{
			WorkspaceID: workspaceID, Provider: inbox.ProviderDemo,
			Email: "gm@demo.test", DisplayName: "GM", LastSyncStatus: inbox.SyncOK,
		}, []inbox.RemoteMessage{{
			ExternalID: "1", Folder: inbox.FolderInbox, FromName: "Sara", FromEmail: "sara@ex.com",
			Subject: "Late checkout", BodyText: "Please allow late checkout.", ReceivedAt: now.Add(-time.Hour),
		}}, nil
}
func (demoBox) Send(_ context.Context, _ id.ID, _ inbox.Draft, _ string) error { return nil }

type emptyOutlook struct{}

func (emptyOutlook) Configured() bool      { return false }
func (emptyOutlook) AuthURL(string) string { return "" }
func (emptyOutlook) Exchange(context.Context, string) (appinbox.TokenPair, appinbox.Profile, error) {
	return appinbox.TokenPair{}, appinbox.Profile{}, nil
}
func (emptyOutlook) Refresh(context.Context, string) (appinbox.TokenPair, error) {
	return appinbox.TokenPair{}, nil
}
func (emptyOutlook) ListInbox(context.Context, string) ([]inbox.RemoteMessage, error) {
	return nil, nil
}
func (emptyOutlook) Send(context.Context, string, inbox.Draft) error { return nil }

func TestDemoConnectListEnrichSendTranslate(t *testing.T) {
	wid := id.New()
	uid := id.New()
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	repo := &memRepo{msgs: map[id.ID]inbox.Message{}}
	svc := appinbox.New(
		repo, passCipher{},
		settingsRepo{s: assistant.Settings{WorkspaceID: wid, Provider: assistant.ProviderOpenAI, Model: "gpt", EncryptedKey: "sealed:sk", DailyLimit: 20, Enabled: true}},
		&usageRepo{}, stubLLM{}, emptyOutlook{}, nil, demoBox{}, fixedClock{now},
		"state-secret-for-tests-32chars!!", "http://localhost:3000",
	)
	ac := actor.Context{
		Principal: ctxkey.Principal{UserID: uid.String()},
		Workspace: tenancy.Workspace{ID: wid, Slug: "duaa-cafe", Timezone: "Asia/Riyadh"},
		Scope:     staffscope.GeneralManager,
	}

	status, err := svc.ConnectDemo(context.Background(), ac)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.UnreadCount < 1 {
		t.Fatalf("%+v", status)
	}
	list, err := svc.List(context.Background(), ac)
	if err != nil || len(list.Messages) < 1 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	msg, err := svc.Enrich(context.Background(), ac, list.Messages[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Category != "guest" || msg.Summary == "" || msg.BodyAR == "" || !msg.Enriched {
		t.Fatalf("%+v", msg)
	}
	tr, err := svc.TranslateCompose(context.Background(), ac, "Merhaba", "en")
	if err != nil || !strings.Contains(tr.Translated, "Dear guest") {
		t.Fatalf("%+v %v", tr, err)
	}
	if err := svc.Send(context.Background(), ac, []string{"guest@example.com"}, "Hi", tr.Translated); err != nil {
		t.Fatal(err)
	}
	if err := svc.Disconnect(context.Background(), ac); err != nil {
		t.Fatal(err)
	}
}
