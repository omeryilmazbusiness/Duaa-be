package inbox

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	listLimit     = 100
	enrichBudget  = 45 * time.Second
	oauthStateTTL = 15 * time.Minute
	enrichMaxTok  = 1800
	composeMaxTok = 1200
)

type Service struct {
	repo     Repository
	cipher   Cipher
	settings SettingsRepo
	usage    UsageRepo
	llm      Completer
	outlook  MailClient
	factory  ClientFactory
	demo     DemoMailbox
	clock    Clock
	stateKey []byte
	public   string
}

func New(
	repo Repository,
	cipher Cipher,
	settings SettingsRepo,
	usage UsageRepo,
	llm Completer,
	outlook MailClient,
	factory ClientFactory,
	demo DemoMailbox,
	clock Clock,
	stateSecret string,
	publicAppURL string,
) *Service {
	key := sha256.Sum256([]byte(stateSecret))
	return &Service{
		repo: repo, cipher: cipher, settings: settings, usage: usage, llm: llm,
		outlook: outlook, factory: factory, demo: demo, clock: clock, stateKey: key[:],
		public: strings.TrimRight(strings.TrimSpace(publicAppURL), "/"),
	}
}

type StatusView struct {
	Connected         bool   `json:"connected"`
	Provider          string `json:"provider,omitempty"`
	Email             string `json:"email,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	LastSyncAt        string `json:"lastSyncAt,omitempty"`
	LastSyncStatus    string `json:"lastSyncStatus,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	UnreadCount       int    `json:"unreadCount"`
	OutlookReady      bool   `json:"outlookReady"`
	HasAppCredentials bool   `json:"hasAppCredentials"`
	AuthURL           string `json:"authUrl,omitempty"`
	RedirectURL       string `json:"redirectUrl,omitempty"`
}

type MessageView struct {
	ID               string `json:"id"`
	ExternalID       string `json:"externalId"`
	Folder           string `json:"folder"`
	FromName         string `json:"fromName"`
	FromEmail        string `json:"fromEmail"`
	ToEmails         string `json:"toEmails"`
	Subject          string `json:"subject"`
	BodyText         string `json:"bodyText"`
	Preview          string `json:"preview"`
	ReceivedAt       string `json:"receivedAt"`
	IsRead           bool   `json:"isRead"`
	HasAttachments   bool   `json:"hasAttachments"`
	Category         string `json:"category"`
	Summary          string `json:"summary"`
	SummaryAR        string `json:"summaryAr"`
	BodyAR           string `json:"bodyAr"`
	DetectedLanguage string `json:"detectedLanguage"`
	Enriched         bool   `json:"enriched"`
}

type ListView struct {
	Status   StatusView    `json:"status"`
	Messages []MessageView `json:"messages"`
}

type TranslateComposeView struct {
	Translated string `json:"translated"`
	TargetLang string `json:"targetLang"`
}

func (s *Service) Status(ctx context.Context, ac actor.Context) (StatusView, error) {
	if err := requireInbox(ac); err != nil {
		return StatusView{}, err
	}
	view := StatusView{RedirectURL: s.redirectURLHint()}
	conn, err := s.repo.GetConnection(ctx, ac.Workspace.ID)
	hasApp := false
	if err == nil {
		hasApp = conn.ClientID != "" && conn.EncryptedClientSecret != ""
		view.HasAppCredentials = hasApp
		view.Connected = strings.TrimSpace(conn.Email) != ""
		view.Provider = string(conn.Provider)
		view.Email = conn.Email
		view.DisplayName = conn.DisplayName
		view.LastSyncStatus = string(conn.LastSyncStatus)
		view.LastError = conn.LastError
		if conn.LastSyncAt != nil {
			view.LastSyncAt = conn.LastSyncAt.UTC().Format(time.RFC3339)
		}
		if view.Connected {
			view.UnreadCount, _ = s.repo.CountUnread(ctx, ac.Workspace.ID)
		}
	} else {
		var xe *errx.Error
		if !(errors.As(err, &xe) && xe.Code == "inbox_not_connected") {
			return StatusView{}, err
		}
	}
	view.OutlookReady = hasApp || (s.outlook != nil && s.outlook.Configured())
	if view.OutlookReady && !view.Connected {
		if auth, aerr := s.StartOutlook(ctx, ac); aerr == nil {
			view.AuthURL = auth
		}
	}
	return view, nil
}

func (s *Service) StartOutlook(ctx context.Context, ac actor.Context) (string, error) {
	if err := requireInbox(ac); err != nil {
		return "", err
	}
	client, _, err := s.mailClient(ctx, ac.Workspace.ID)
	if err != nil {
		return "", err
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return "", errx.Unauthenticated("invalid token")
	}
	state, err := s.signState(oauthState{
		WorkspaceID: ac.Workspace.ID.String(),
		UserID:      uid.String(),
		Slug:        ac.Workspace.Slug,
		Exp:         s.clock.Now().UTC().Add(oauthStateTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	return client.AuthURL(state), nil
}

func (s *Service) CompleteOutlook(ctx context.Context, code, state string) (string, error) {
	payload, err := s.verifyState(state)
	if err != nil {
		return s.public + "/login?outlook=error", err
	}
	wid, err := id.Parse(payload.WorkspaceID)
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	uid, err := id.Parse(payload.UserID)
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	client, existing, err := s.mailClient(ctx, wid)
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	tokens, profile, err := client.Exchange(ctx, code)
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	now := s.clock.Now().UTC()
	access, err := s.cipher.Seal(tokens.AccessToken, wid.String())
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	refresh, err := s.cipher.Seal(tokens.RefreshToken, wid.String())
	if err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	exp := tokens.ExpiresAt
	conn := inbox.Connection{
		WorkspaceID: wid, Provider: inbox.ProviderOutlook,
		Email: profile.Email, DisplayName: profile.DisplayName,
		EncryptedAccessToken: access, EncryptedRefreshToken: refresh,
		TokenExpiresAt: &exp, Scopes: tokens.Scopes,
		LastSyncStatus: inbox.SyncNever, ConnectedBy: uid,
		CreatedAt: now, UpdatedAt: now,
	}
	if existing != nil {
		conn.ClientID = existing.ClientID
		conn.EncryptedClientSecret = existing.EncryptedClientSecret
		conn.Tenant = existing.Tenant
		if !existing.CreatedAt.IsZero() {
			conn.CreatedAt = existing.CreatedAt
		}
	}
	if err := s.repo.SaveConnection(ctx, conn); err != nil {
		return s.redirect(payload.Slug, "error"), err
	}
	_, _ = s.syncConnection(ctx, actorStub(wid, uid, payload.Slug), conn, true)
	return s.redirect(payload.Slug, "connected"), nil
}

func (s *Service) ConnectDemo(ctx context.Context, ac actor.Context) (StatusView, error) {
	if err := requireInbox(ac); err != nil {
		return StatusView{}, err
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return StatusView{}, errx.Unauthenticated("invalid token")
	}
	conn, remotes, err := s.demo.Connect(ctx, ac.Workspace.ID)
	if err != nil {
		return StatusView{}, err
	}
	now := s.clock.Now().UTC()
	conn.ConnectedBy = uid
	conn.CreatedAt = now
	conn.UpdatedAt = now
	conn.LastSyncAt = &now
	conn.LastSyncStatus = inbox.SyncOK
	if err := s.repo.SaveConnection(ctx, conn); err != nil {
		return StatusView{}, err
	}
	_ = s.repo.DeleteMessages(ctx, ac.Workspace.ID)
	for _, remote := range remotes {
		msg := fromRemote(ac.Workspace.ID, remote, now)
		if err := s.repo.UpsertMessage(ctx, msg); err != nil {
			return StatusView{}, err
		}
	}
	return s.Status(ctx, ac)
}

func (s *Service) Disconnect(ctx context.Context, ac actor.Context) error {
	if err := requireInbox(ac); err != nil {
		return err
	}
	_ = s.repo.DeleteMessages(ctx, ac.Workspace.ID)
	return s.repo.DeleteConnection(ctx, ac.Workspace.ID)
}

func (s *Service) Sync(ctx context.Context, ac actor.Context) (ListView, error) {
	if err := requireInbox(ac); err != nil {
		return ListView{}, err
	}
	conn, err := s.repo.GetConnection(ctx, ac.Workspace.ID)
	if err != nil {
		return ListView{}, err
	}
	if _, err := s.syncConnection(ctx, ac, conn, false); err != nil {
		return ListView{}, err
	}
	return s.List(ctx, ac)
}

func (s *Service) List(ctx context.Context, ac actor.Context) (ListView, error) {
	if err := requireInbox(ac); err != nil {
		return ListView{}, err
	}
	status, err := s.Status(ctx, ac)
	if err != nil {
		return ListView{}, err
	}
	if !status.Connected {
		return ListView{Status: status, Messages: []MessageView{}}, nil
	}
	msgs, err := s.repo.ListMessages(ctx, ac.Workspace.ID, listLimit)
	if err != nil {
		return ListView{}, err
	}
	out := make([]MessageView, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toView(m))
	}
	return ListView{Status: status, Messages: out}, nil
}

func (s *Service) Get(ctx context.Context, ac actor.Context, messageID string) (MessageView, error) {
	if err := requireInbox(ac); err != nil {
		return MessageView{}, err
	}
	mid, err := id.Parse(messageID)
	if err != nil {
		return MessageView{}, errx.Invalid("invalid_message_id", "invalid message id")
	}
	msg, err := s.repo.GetMessage(ctx, ac.Workspace.ID, mid)
	if err != nil {
		return MessageView{}, err
	}
	return toView(msg), nil
}

func (s *Service) Enrich(ctx context.Context, ac actor.Context, messageID string) (MessageView, error) {
	ctx, cancel := context.WithTimeout(ctx, enrichBudget)
	defer cancel()
	if err := requireInbox(ac); err != nil {
		return MessageView{}, err
	}
	if err := requireAI(ac); err != nil {
		return MessageView{}, err
	}
	mid, err := id.Parse(messageID)
	if err != nil {
		return MessageView{}, errx.Invalid("invalid_message_id", "invalid message id")
	}
	msg, err := s.repo.GetMessage(ctx, ac.Workspace.ID, mid)
	if err != nil {
		return MessageView{}, err
	}
	enriched, err := s.enrichMessage(ctx, ac, msg)
	if err != nil {
		return MessageView{}, err
	}
	return toView(enriched), nil
}

func (s *Service) TranslateCompose(ctx context.Context, ac actor.Context, text, targetLang string) (TranslateComposeView, error) {
	ctx, cancel := context.WithTimeout(ctx, enrichBudget)
	defer cancel()
	if err := requireInbox(ac); err != nil {
		return TranslateComposeView{}, err
	}
	if err := requireAI(ac); err != nil {
		return TranslateComposeView{}, err
	}
	lang, err := inbox.ParseComposeLang(targetLang)
	if err != nil {
		return TranslateComposeView{}, err
	}
	body := strings.TrimSpace(text)
	if body == "" {
		return TranslateComposeView{}, errx.Invalid("missing_body", "message body is required")
	}
	if len([]rune(body)) > inbox.MaxBody {
		return TranslateComposeView{}, errx.Invalid("body_too_long", "message body is too long")
	}
	cur, key, err := s.activeKey(ctx, ac)
	if err != nil {
		return TranslateComposeView{}, err
	}
	date := businessDate(ac, s.clock.Now())
	used, ok, err := s.usage.ReserveAIUsage(ctx, ac.Workspace.ID, date, cur.DailyLimit)
	if err != nil {
		return TranslateComposeView{}, err
	}
	if !ok {
		return TranslateComposeView{}, errx.RateLimited("ai_daily_limit", "the daily AI question limit has been reached; it resets at midnight")
	}
	_ = used
	targetName := "English"
	if lang == inbox.ComposeTR {
		targetName = "Turkish"
	}
	system := fmt.Sprintf(`You are an executive hospitality communications editor for a hotel/cafe general manager.
The source may be Arabic or another language. Rewrite it into ultra-professional, polished %s for external business email.
Preserve meaning, names, dates, amounts and commitments. Do not invent facts. Keep a warm but formal hospitality voice.
Do not add a subject line. Return JSON only: {"translated":"..."}`, targetName)
	res, err := s.completeLLM(ctx, cur, key, system, body, composeMaxTok)
	if err != nil {
		_ = s.usage.ReleaseAIUsage(context.WithoutCancel(ctx), ac.Workspace.ID, date)
		return TranslateComposeView{}, err
	}
	translated := parseTranslated(res.Text, body)
	return TranslateComposeView{Translated: translated, TargetLang: string(lang)}, nil
}

func (s *Service) Send(ctx context.Context, ac actor.Context, to []string, subject, body string) error {
	if err := requireInbox(ac); err != nil {
		return err
	}
	draft, err := inbox.NormalizeDraft(to, subject, body)
	if err != nil {
		return err
	}
	conn, err := s.repo.GetConnection(ctx, ac.Workspace.ID)
	if err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	switch conn.Provider {
	case inbox.ProviderDemo:
		if err := s.demo.Send(ctx, ac.Workspace.ID, draft, conn.Email); err != nil {
			return err
		}
	case inbox.ProviderOutlook:
		access, err := s.accessToken(ctx, &conn)
		if err != nil {
			return err
		}
		client, _, err := s.mailClient(ctx, ac.Workspace.ID)
		if err != nil {
			return err
		}
		if err := client.Send(ctx, access, draft); err != nil {
			return err
		}
	default:
		return errx.Upstream("inbox_provider", "unsupported mailbox provider")
	}
	msg := inbox.Message{
		ID: id.New(), WorkspaceID: ac.Workspace.ID,
		ExternalID: "sent-" + id.New().String(), Folder: inbox.FolderSent,
		FromName: conn.DisplayName, FromEmail: conn.Email,
		ToEmails: strings.Join(draft.To, ", "), Subject: draft.Subject,
		BodyText: draft.Body, Preview: inbox.PreviewOf(draft.Body),
		ReceivedAt: now, IsRead: true, Category: inbox.CatInternal,
		CreatedAt: now, UpdatedAt: now,
	}
	return s.repo.UpsertMessage(ctx, msg)
}

func (s *Service) syncConnection(ctx context.Context, ac actor.Context, conn inbox.Connection, first bool) (inbox.Connection, error) {
	now := s.clock.Now().UTC()
	var remotes []inbox.RemoteMessage
	var err error
	switch conn.Provider {
	case inbox.ProviderDemo:
		_, remotes, err = s.demo.Connect(ctx, ac.Workspace.ID)
	case inbox.ProviderOutlook:
		var access string
		var client MailClient
		access, err = s.accessToken(ctx, &conn)
		if err == nil {
			client, _, err = s.mailClient(ctx, ac.Workspace.ID)
		}
		if err == nil {
			remotes, err = client.ListInbox(ctx, access)
		}
	default:
		err = errx.Upstream("inbox_provider", "unsupported mailbox provider")
	}
	if err != nil {
		conn.LastSyncStatus = inbox.SyncError
		conn.LastError = err.Error()
		conn.UpdatedAt = now
		_ = s.repo.SaveConnection(ctx, conn)
		return conn, err
	}
	for _, remote := range remotes {
		existing, gerr := s.findByExternal(ctx, ac.Workspace.ID, remote.ExternalID)
		msg := fromRemote(ac.Workspace.ID, remote, now)
		if gerr == nil {
			msg.ID = existing.ID
			msg.Category = existing.Category
			msg.Summary = existing.Summary
			msg.SummaryAR = existing.SummaryAR
			msg.BodyAR = existing.BodyAR
			msg.DetectedLanguage = existing.DetectedLanguage
			msg.EnrichedAt = existing.EnrichedAt
			msg.CreatedAt = existing.CreatedAt
		}
		if err := s.repo.UpsertMessage(ctx, msg); err != nil {
			return conn, err
		}
	}
	conn.LastSyncAt = &now
	conn.LastSyncStatus = inbox.SyncOK
	conn.LastError = ""
	conn.UpdatedAt = now
	if err := s.repo.SaveConnection(ctx, conn); err != nil {
		return conn, err
	}
	s.autoEnrich(ctx, ac)
	_ = first
	return conn, nil
}

func (s *Service) findByExternal(ctx context.Context, wid id.ID, external string) (inbox.Message, error) {
	msgs, err := s.repo.ListMessages(ctx, wid, listLimit)
	if err != nil {
		return inbox.Message{}, err
	}
	for _, m := range msgs {
		if m.ExternalID == external {
			return m, nil
		}
	}
	return inbox.Message{}, inbox.MessageNotFoundError()
}

func (s *Service) enrichMessage(ctx context.Context, ac actor.Context, msg inbox.Message) (inbox.Message, error) {
	cur, key, err := s.activeKey(ctx, ac)
	if err != nil {
		return inbox.Message{}, err
	}
	date := businessDate(ac, s.clock.Now())
	_, ok, err := s.usage.ReserveAIUsage(ctx, ac.Workspace.ID, date, cur.DailyLimit)
	if err != nil {
		return inbox.Message{}, err
	}
	if !ok {
		return inbox.Message{}, errx.RateLimited("ai_daily_limit", "the daily AI question limit has been reached; it resets at midnight")
	}
	system := `You are the executive inbox AI for a hotel/cafe general manager.
Classify the email, write a brief Arabic summary for the inbox list, summarise in English, and translate the FULL body into Modern Standard Arabic.
Categories (pick one): guest, reservation, supplier, staff, finance, complaint, marketing, internal, other.
Return JSON only:
{"category":"...","summary":"...","summaryAr":"...","bodyAr":"...","detectedLanguage":"en|ar|tr|..."}
summary: 2 short executive sentences in English.
summaryAr: one short Arabic sentence (max ~140 characters) for the mail list row.
bodyAr: complete Arabic translation of the email body.`
	user := fmt.Sprintf("From: %s <%s>\nSubject: %s\n\n%s", msg.FromName, msg.FromEmail, msg.Subject, msg.BodyText)
	res, err := s.completeLLM(ctx, cur, key, system, user, enrichMaxTok)
	if err != nil {
		_ = s.usage.ReleaseAIUsage(context.WithoutCancel(ctx), ac.Workspace.ID, date)
		return inbox.Message{}, err
	}
	parsed := parseEnrichment(res.Text)
	now := s.clock.Now().UTC()
	if cat, cerr := inbox.ParseCategory(parsed.Category); cerr == nil && cat != "" {
		msg.Category = cat
	} else if msg.Category == "" {
		msg.Category = inbox.CatOther
	}
	if parsed.Summary != "" {
		msg.Summary = truncateRunes(parsed.Summary, inbox.MaxSummary)
	}
	if parsed.SummaryAR != "" {
		msg.SummaryAR = truncateRunes(parsed.SummaryAR, 280)
	}
	if parsed.BodyAR != "" {
		msg.BodyAR = truncateRunes(parsed.BodyAR, inbox.MaxBody)
	}
	msg.DetectedLanguage = strings.ToLower(strings.TrimSpace(parsed.DetectedLanguage))
	msg.EnrichedAt = &now
	msg.UpdatedAt = now
	if err := s.repo.UpsertMessage(ctx, msg); err != nil {
		return inbox.Message{}, err
	}
	return msg, nil
}

func (s *Service) accessToken(ctx context.Context, conn *inbox.Connection) (string, error) {
	aad := conn.WorkspaceID.String()
	now := s.clock.Now().UTC()
	if conn.TokenExpiresAt != nil && conn.TokenExpiresAt.After(now.Add(2*time.Minute)) {
		return s.cipher.Open(conn.EncryptedAccessToken, aad)
	}
	refresh, err := s.cipher.Open(conn.EncryptedRefreshToken, aad)
	if err != nil {
		return "", errx.Upstream("outlook_token", "Outlook session expired; reconnect the mailbox")
	}
	client, _, cerr := s.mailClient(ctx, conn.WorkspaceID)
	if cerr != nil {
		return "", cerr
	}
	tokens, err := client.Refresh(ctx, refresh)
	if err != nil {
		return "", errx.Upstream("outlook_token", "Outlook session expired; reconnect the mailbox")
	}
	accessSeal, err := s.cipher.Seal(tokens.AccessToken, aad)
	if err != nil {
		return "", err
	}
	refreshSeal, err := s.cipher.Seal(tokens.RefreshToken, aad)
	if err != nil {
		return "", err
	}
	exp := tokens.ExpiresAt
	conn.EncryptedAccessToken = accessSeal
	conn.EncryptedRefreshToken = refreshSeal
	conn.TokenExpiresAt = &exp
	conn.UpdatedAt = now
	if err := s.repo.SaveConnection(ctx, *conn); err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (s *Service) activeKey(ctx context.Context, ac actor.Context) (assistant.Settings, string, error) {
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	if err != nil {
		return assistant.Settings{}, "", errx.Upstream("ai_not_configured", "connect the DUAA AI assistant in Settings before using inbox AI")
	}
	if !cur.Enabled || strings.TrimSpace(cur.EncryptedKey) == "" {
		return assistant.Settings{}, "", errx.Upstream("ai_not_configured", "connect the DUAA AI assistant in Settings before using inbox AI")
	}
	key, err := s.cipher.Open(cur.EncryptedKey, ac.Workspace.ID.String())
	if err != nil {
		return assistant.Settings{}, "", errx.Upstream("ai_key_unreadable", "the stored AI key could not be opened; re-enter it in Settings")
	}
	return cur, key, nil
}

func (s *Service) completeLLM(ctx context.Context, cur assistant.Settings, key, system, user string, maxTok int) (CompletionResult, error) {
	req := CompletionRequest{
		Provider: cur.Provider, Model: cur.Model, APIKey: key, System: system,
		Messages:        []assistant.Message{{Role: assistant.RoleUser, Content: user}},
		MaxOutputTokens: maxTok, Temperature: 0.2,
	}
	res, err := s.llm.Complete(ctx, req)
	if err == nil || !recoverableLLM(err) {
		return res, err
	}
	tried := map[string]struct{}{cur.Model: {}}
	model := cur.Model
	for i := 0; i < 3; i++ {
		next := assistant.FastFallback(cur.Provider, model)
		if next == "" {
			break
		}
		if _, seen := tried[next]; seen {
			break
		}
		tried[next] = struct{}{}
		req.Model = next
		res, err = s.llm.Complete(ctx, req)
		if err == nil {
			return res, nil
		}
		if !recoverableLLM(err) {
			return CompletionResult{}, err
		}
		model = next
	}
	return CompletionResult{}, err
}

func recoverableLLM(err error) bool {
	var xe *errx.Error
	if !errors.As(err, &xe) {
		return false
	}
	switch xe.Code {
	case "ai_model_not_found", "ai_request_rejected", "ai_empty_response", "ai_provider_unavailable":
		return true
	default:
		return false
	}
}

type oauthState struct {
	WorkspaceID string `json:"w"`
	UserID      string `json:"u"`
	Slug        string `json:"s"`
	Exp         int64  `json:"e"`
}

func (s *Service) signState(st oauthState) (string, error) {
	raw, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (s *Service) verifyState(state string) (oauthState, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 2 {
		return oauthState{}, errx.Invalid("invalid_oauth_state", "invalid Outlook state")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthState{}, errx.Invalid("invalid_oauth_state", "invalid Outlook state")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oauthState{}, errx.Invalid("invalid_oauth_state", "invalid Outlook state")
	}
	mac := hmac.New(sha256.New, s.stateKey)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return oauthState{}, errx.Invalid("invalid_oauth_state", "invalid Outlook state")
	}
	var st oauthState
	if err := json.Unmarshal(raw, &st); err != nil {
		return oauthState{}, errx.Invalid("invalid_oauth_state", "invalid Outlook state")
	}
	if time.Now().UTC().Unix() > st.Exp {
		return oauthState{}, errx.Invalid("oauth_state_expired", "Outlook login expired; try again")
	}
	return st, nil
}

func (s *Service) redirect(slug, result string) string {
	base := s.public
	if slug != "" {
		return fmt.Sprintf("%s/%s/manager?tab=inbox&outlook=%s", base, url.PathEscape(slug), url.QueryEscape(result))
	}
	return base + "/login?outlook=" + url.QueryEscape(result)
}

func (s *Service) autoEnrich(ctx context.Context, ac actor.Context) {
	if requireAI(ac) != nil {
		return
	}
	if _, _, err := s.activeKey(ctx, ac); err != nil {
		return
	}
	msgs, err := s.repo.ListMessages(ctx, ac.Workspace.ID, 12)
	if err != nil {
		return
	}
	n := 0
	for _, msg := range msgs {
		if msg.Folder != inbox.FolderInbox {
			continue
		}
		if msg.SummaryAR != "" && msg.BodyAR != "" {
			continue
		}
		if _, err := s.enrichMessage(ctx, ac, msg); err != nil {
			return
		}
		n++
		if n >= 5 {
			return
		}
	}
}

func requireInbox(ac actor.Context) error {
	if !rbac.Require(ac.Scope, rbac.ViewInbox) {
		return errx.Forbidden("forbidden", "only the general manager can open the inbox")
	}
	return nil
}

func requireAI(ac actor.Context) error {
	if !rbac.Require(ac.Scope, rbac.UseAI) {
		return errx.Forbidden("forbidden", "AI tools are limited to the general manager")
	}
	return nil
}

func actorStub(wid, uid id.ID, slug string) actor.Context {
	return actor.Context{
		Principal: ctxkey.Principal{UserID: uid.String()},
		Workspace: tenancy.Workspace{ID: wid, Slug: slug},
	}
}

func fromRemote(wid id.ID, remote inbox.RemoteMessage, now time.Time) inbox.Message {
	folder := remote.Folder
	if folder == "" {
		folder = inbox.FolderInbox
	}
	return inbox.Message{
		ID: id.New(), WorkspaceID: wid, ExternalID: remote.ExternalID,
		ConversationID: remote.ConversationID, Folder: folder,
		FromName: remote.FromName, FromEmail: remote.FromEmail, ToEmails: remote.ToEmails,
		Subject: remote.Subject, BodyText: remote.BodyText, Preview: inbox.PreviewOf(remote.BodyText),
		ReceivedAt: remote.ReceivedAt, IsRead: remote.IsRead, HasAttachments: remote.HasAttachments,
		CreatedAt: now, UpdatedAt: now,
	}
}

func toView(m inbox.Message) MessageView {
	return MessageView{
		ID: m.ID.String(), ExternalID: m.ExternalID, Folder: string(m.Folder),
		FromName: m.FromName, FromEmail: m.FromEmail, ToEmails: m.ToEmails,
		Subject: m.Subject, BodyText: m.BodyText, Preview: m.Preview,
		ReceivedAt: m.ReceivedAt.UTC().Format(time.RFC3339), IsRead: m.IsRead,
		HasAttachments: m.HasAttachments, Category: string(m.Category),
		Summary: m.Summary, SummaryAR: m.SummaryAR, BodyAR: m.BodyAR, DetectedLanguage: m.DetectedLanguage,
		Enriched: m.EnrichedAt != nil && (m.SummaryAR != "" || m.BodyAR != "" || m.Summary != ""),
	}
}

func businessDate(ac actor.Context, now time.Time) string {
	loc := time.UTC
	if tz := strings.TrimSpace(ac.Workspace.Timezone); tz != "" {
		if loaded, err := time.LoadLocation(tz); err == nil {
			loc = loaded
		}
	}
	return now.In(loc).Format("2006-01-02")
}

type enrichDTO struct {
	Category         string `json:"category"`
	Summary          string `json:"summary"`
	SummaryAR        string `json:"summaryAr"`
	BodyAR           string `json:"bodyAr"`
	DetectedLanguage string `json:"detectedLanguage"`
}

func parseEnrichment(raw string) enrichDTO {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	var out enrichDTO
	if json.Unmarshal([]byte(trimmed), &out) == nil {
		return out
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		_ = json.Unmarshal([]byte(trimmed[start:end+1]), &out)
	}
	return out
}

func parseTranslated(raw, fallback string) string {
	var parsed struct {
		Translated string `json:"translated"`
	}
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if json.Unmarshal([]byte(trimmed), &parsed) == nil && strings.TrimSpace(parsed.Translated) != "" {
		return strings.TrimSpace(parsed.Translated)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start && json.Unmarshal([]byte(trimmed[start:end+1]), &parsed) == nil && strings.TrimSpace(parsed.Translated) != "" {
		return strings.TrimSpace(parsed.Translated)
	}
	if trimmed != "" {
		return trimmed
	}
	return fallback
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
