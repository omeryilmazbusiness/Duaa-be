package assistant

import (
	"context"
	"errors"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/assistant"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	// Whole Chat() must finish before the Next.js rewrite (~30s) drops the
	// socket; otherwise the widget stays on “thinking” forever.
	chatBudget      = 16 * time.Second
	chatTimeout     = 13 * time.Second
	setupBudget     = 10 * time.Second
	verifyAttempt   = 4 * time.Second
	listTimeout     = 6 * time.Second
	chatMaxTokens   = 180
	chatTemperature = 0.2
	verifyMaxTokens = 16
)

type Service struct {
	settings SettingsRepo
	usage    UsageRepo
	cipher   Cipher
	llm      Completer
	snapshot ContextProvider
	models   ModelLister
	clock    clock.Clock
}

func New(settings SettingsRepo, usage UsageRepo, cipher Cipher, llm Completer, snapshot ContextProvider, models ModelLister, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{settings: settings, usage: usage, cipher: cipher, llm: llm, snapshot: snapshot, models: models, clock: clk}
}

type ModelOption struct {
	ID   string `json:"id"`
	Free bool   `json:"free"`
}

type ProviderOption struct {
	ID           string        `json:"id"`
	Label        string        `json:"label"`
	DefaultModel string        `json:"defaultModel"`
	Models       []ModelOption `json:"models"`
}

// SettingsView never carries the API key, only whether one is stored.
type SettingsView struct {
	Configured bool             `json:"configured"`
	Enabled    bool             `json:"enabled"`
	Provider   string           `json:"provider"`
	Model      string           `json:"model"`
	KeyHint    string           `json:"keyHint,omitempty"`
	DailyLimit int              `json:"dailyLimit"`
	UpdatedAt  string           `json:"updatedAt,omitempty"`
	Providers  []ProviderOption `json:"providers"`
}

type UsageView struct {
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
	Remaining int    `json:"remaining"`
	ResetsAt  string `json:"resetsAt"`
}

type StatusView struct {
	Available  bool      `json:"available"`
	Configured bool      `json:"configured"`
	Enabled    bool      `json:"enabled"`
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	Usage      UsageView `json:"usage"`
}

type ChatView struct {
	Reply string    `json:"reply"`
	Usage UsageView `json:"usage"`
}

type SaveInput struct {
	Provider   string
	Model      string
	APIKey     string // empty keeps the stored key
	DailyLimit int
	Enabled    bool
}

type ChatInput struct {
	Messages []assistant.Message
	Locale   string
}

func (s *Service) GetSettings(ctx context.Context, ac actor.Context) (SettingsView, error) {
	if err := requireAdmin(ac); err != nil {
		return SettingsView{}, err
	}
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return defaultSettingsView(), nil
		}
		return SettingsView{}, err
	}
	return toSettingsView(cur), nil
}

func (s *Service) SaveSettings(ctx context.Context, ac actor.Context, in SaveInput) (SettingsView, error) {
	if err := requireAdmin(ac); err != nil {
		return SettingsView{}, err
	}
	provider, err := assistant.ParseProvider(in.Provider)
	if err != nil {
		return SettingsView{}, err
	}
	model, err := assistant.NormalizeModel(provider, in.Model)
	if err != nil {
		return SettingsView{}, err
	}
	limit, err := assistant.NormalizeDailyLimit(in.DailyLimit)
	if err != nil {
		return SettingsView{}, err
	}

	userID, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return SettingsView{}, errx.Unauthenticated("invalid user")
	}
	now := s.clock.Now()
	next := assistant.Settings{
		WorkspaceID: ac.Workspace.ID, Provider: provider, Model: model, DailyLimit: limit,
		Enabled: in.Enabled, UpdatedByUserID: userID, CreatedAt: now, UpdatedAt: now,
	}
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	hasCur := err == nil
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return SettingsView{}, err
	}
	if hasCur {
		next.CreatedAt = cur.CreatedAt
		next.EncryptedKey, next.KeyHint = cur.EncryptedKey, cur.KeyHint
	}

	if in.APIKey != "" {
		key, err := assistant.NormalizeAPIKey(in.APIKey)
		if err != nil {
			return SettingsView{}, err
		}
		// Reject only a provider-confirmed bad key. Timeouts, regional
		// blocks and model 404s must not 502 the Next.js rewrite or
		// force the GM to paste the key again.
		resolved, err := s.resolveAndVerify(ctx, provider, model, key)
		if err != nil {
			return SettingsView{}, err
		}
		next.Model = resolved
		sealed, err := s.cipher.Seal(key, ac.Workspace.ID.String())
		if err != nil {
			return SettingsView{}, err
		}
		next.EncryptedKey, next.KeyHint = sealed, assistant.KeyHint(key)
	} else if !hasCur {
		return SettingsView{}, errx.Invalid("ai_key_required", "an API key is required for first setup")
	} else if cur.Provider != provider {
		return SettingsView{}, errx.Invalid("ai_key_required", "enter a new API key when changing provider")
	}

	if err := s.settings.SaveAssistantSettings(ctx, next); err != nil {
		return SettingsView{}, err
	}
	return toSettingsView(next), nil
}

// Models lists the models selectable for the configured provider. With a
// stored key it asks the provider for the live list (so every current version
// appears); otherwise, or if that fails, it serves the built-in catalogue.
func (s *Service) Models(ctx context.Context, ac actor.Context) ([]ModelOption, error) {
	if err := requireAdmin(ac); err != nil {
		return nil, err
	}
	provider := assistant.DefaultProvider
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	hasKey := err == nil && cur.EncryptedKey != ""
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return nil, err
	}
	if hasKey {
		provider = cur.Provider
	}
	static, _ := assistant.InfoFor(provider)
	if !hasKey || s.models == nil {
		return modelOptions(static.Models), nil
	}
	key, err := s.cipher.Open(cur.EncryptedKey, ac.Workspace.ID.String())
	if err != nil {
		return modelOptions(static.Models), nil
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	live, err := s.models.ListModels(ctx, provider, key)
	if err != nil || len(live) == 0 {
		return modelOptions(static.Models), nil
	}
	return modelOptions(assistant.MergeModels(static.Models, live)), nil
}

// RemoveKey deletes the stored key and configuration.
func (s *Service) RemoveKey(ctx context.Context, ac actor.Context) error {
	if err := requireAdmin(ac); err != nil {
		return err
	}
	return s.settings.DeleteAssistantSettings(ctx, ac.Workspace.ID)
}

func (s *Service) Status(ctx context.Context, ac actor.Context) (StatusView, error) {
	if err := requireUse(ac); err != nil {
		return StatusView{}, err
	}
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return StatusView{Usage: s.usageView(ac, 0, assistant.DefaultDailyLimit)}, nil
		}
		return StatusView{}, err
	}
	used, err := s.usage.AIUsage(ctx, ac.Workspace.ID, s.businessDate(ac))
	if err != nil {
		return StatusView{}, err
	}
	return StatusView{
		Available: cur.Available(), Configured: cur.EncryptedKey != "", Enabled: cur.Enabled,
		Provider: string(cur.Provider), Model: cur.Model, Usage: s.usageView(ac, used, cur.DailyLimit),
	}, nil
}

func (s *Service) Chat(ctx context.Context, ac actor.Context, in ChatInput) (ChatView, error) {
	ctx, cancel := context.WithTimeout(ctx, chatBudget)
	defer cancel()
	if err := requireUse(ac); err != nil {
		return ChatView{}, err
	}
	msgs, err := assistant.NormalizeConversation(in.Messages)
	if err != nil {
		return ChatView{}, err
	}
	cur, err := s.settings.GetAssistantSettings(ctx, ac.Workspace.ID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return ChatView{}, errx.Conflict("ai_not_configured", "the AI assistant is not set up yet")
		}
		return ChatView{}, err
	}
	if !cur.Available() {
		return ChatView{}, errx.Conflict("ai_not_configured", "the AI assistant is not enabled")
	}
	key, err := s.cipher.Open(cur.EncryptedKey, ac.Workspace.ID.String())
	if err != nil {
		return ChatView{}, errx.Conflict("ai_key_unreadable", "the stored API key can no longer be read; enter it again in Settings")
	}

	date := s.businessDate(ac)
	used, ok, err := s.usage.ReserveAIUsage(ctx, ac.Workspace.ID, date, cur.DailyLimit)
	if err != nil {
		return ChatView{}, err
	}
	if !ok {
		return ChatView{}, errx.RateLimited("ai_daily_limit", "the daily AI question limit has been reached; it resets at midnight")
	}

	reply, err := s.complete(ctx, ac, cur, key, msgs, in.Locale, date)
	if recoverableChat(err) {
		if next := assistant.FastFallback(cur.Provider, cur.Model); next != "" {
			trial := cur
			trial.Model = next
			reply, err = s.complete(ctx, ac, trial, key, msgs, in.Locale, date)
			if err == nil {
				cur.Model = next
				cur.UpdatedAt = s.clock.Now()
				_ = s.settings.SaveAssistantSettings(context.WithoutCancel(ctx), cur)
			}
		}
		if recoverableChat(err) {
			if next, ok := s.recoverModel(ctx, cur, key); ok {
				cur.Model = next
				reply, err = s.complete(ctx, ac, cur, key, msgs, in.Locale, date)
				if err == nil {
					cur.UpdatedAt = s.clock.Now()
					_ = s.settings.SaveAssistantSettings(context.WithoutCancel(ctx), cur)
				}
			}
		}
	}
	if err != nil {
		// A failed call must not burn the daily quota.
		_ = s.usage.ReleaseAIUsage(context.WithoutCancel(ctx), ac.Workspace.ID, date)
		return ChatView{}, err
	}
	return ChatView{Reply: reply, Usage: s.usageView(ac, used, cur.DailyLimit)}, nil
}

// recoverModel finds another generate-capable id when the stored catalogue
// alias 404s. The live list is not trusted on its own — Gemini lists
// gemini-2.5-flash even when generateContent rejects it.
func (s *Service) recoverModel(ctx context.Context, cur assistant.Settings, key string) (string, bool) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < verifyAttempt {
		return "", false
	}
	info, _ := assistant.InfoFor(cur.Provider)
	live, err := s.listLive(ctx, cur.Provider, key)
	if err != nil && errorCode(err) == "ai_key_rejected" {
		return "", false
	}
	for _, model := range assistant.SetupTries(cur.Model, info.Models, live) {
		if model == cur.Model {
			continue
		}
		if err := s.verify(ctx, cur.Provider, model, key); err == nil {
			return model, true
		}
	}
	return "", false
}

func (s *Service) complete(ctx context.Context, ac actor.Context, cur assistant.Settings, key string, msgs []assistant.Message, locale, date string) (string, error) {
	snapshot, err := s.snapshot.Snapshot(ctx, ac)
	if err != nil {
		snapshot = "Workspace data is temporarily unavailable. Say so if the user asks about numbers."
	}
	system := assistant.SystemPrompt(assistant.PromptInput{
		WorkspaceName: ac.Workspace.Name, WorkspaceKind: string(ac.Workspace.Kind),
		BusinessDate: date, Locale: locale, Snapshot: snapshot,
	})
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	res, err := s.llm.Complete(ctx, CompletionRequest{
		Provider: cur.Provider, Model: cur.Model, APIKey: key, System: system,
		Messages: msgs, MaxOutputTokens: chatMaxTokens, Temperature: chatTemperature,
	})
	if err != nil {
		return "", err
	}
	if res.Text == "" {
		return "", errx.Upstream("ai_empty_response", "the AI provider returned an empty answer; try rephrasing")
	}
	return res.Text, nil
}

func (s *Service) verify(ctx context.Context, p assistant.Provider, model, key string) error {
	ctx, cancel := context.WithTimeout(ctx, verifyAttempt)
	defer cancel()
	_, err := s.llm.Complete(ctx, CompletionRequest{
		Provider: p, Model: model, APIKey: key, System: "Reply with the single word: ok",
		Messages:        []assistant.Message{{Role: assistant.RoleUser, Content: "ping"}},
		MaxOutputTokens: verifyMaxTokens,
	})
	return err
}

// resolveAndVerify picks a model this key can use. A live model list is
// enough to prove the key; generate is best-effort so a stalled Gemini
// call cannot turn setup into a proxy 502.
func (s *Service) resolveAndVerify(ctx context.Context, p assistant.Provider, wanted, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, setupBudget)
	defer cancel()
	info, _ := assistant.InfoFor(p)
	live, err := s.listLive(ctx, p, key)
	if errorCode(err) == "ai_key_rejected" {
		return "", err
	}
	fallback := wanted
	if len(live) > 0 {
		fallback = assistant.ResolveModel(wanted, info.Models, live)
		// Listing succeeded, so the key is valid. Skip generate — 2.5
		// thinking models regularly 404 the catalogue alias or hang.
		return fallback, nil
	}
	for _, model := range assistant.SetupTries(wanted, info.Models, live) {
		if err := s.verify(ctx, p, model, key); err == nil {
			return model, nil
		} else if errorCode(err) == "ai_key_rejected" {
			return "", err
		} else if !retryableModel(err) {
			break
		}
	}
	return fallback, nil
}

func (s *Service) listLive(ctx context.Context, p assistant.Provider, key string) ([]string, error) {
	if s.models == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	return s.models.ListModels(ctx, p, key)
}

func errorCode(err error) string {
	var xe *errx.Error
	if errors.As(err, &xe) {
		return xe.Code
	}
	return ""
}

func retryableModel(err error) bool {
	switch errorCode(err) {
	case "ai_model_not_found", "ai_request_rejected", "ai_empty_response":
		return true
	default:
		return false
	}
}

func recoverableChat(err error) bool {
	switch errorCode(err) {
	case "ai_model_not_found", "ai_request_rejected", "ai_empty_response":
		return true
	default:
		return false
	}
}

func (s *Service) businessDate(ac actor.Context) string {
	return finance.BusinessDate(s.clock.Now(), ac.Workspace.Timezone)
}

func (s *Service) usageView(ac actor.Context, used, limit int) UsageView {
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return UsageView{
		Used: used, Limit: limit, Remaining: remaining,
		ResetsAt: assistant.NextReset(s.clock.Now(), ac.Workspace.Timezone).Format(time.RFC3339),
	}
}

func requireAdmin(ac actor.Context) error {
	if !rbac.Allows(ac.Scope, rbac.ManageWorkspace) || !rbac.Allows(ac.Scope, rbac.UseAI) {
		return errx.Forbidden("forbidden", "only the general manager can configure the AI assistant")
	}
	return nil
}

func requireUse(ac actor.Context) error {
	if !rbac.Allows(ac.Scope, rbac.UseAI) {
		return errx.Forbidden("forbidden", "the AI assistant is available to the general manager only")
	}
	return nil
}

func providerOptions() []ProviderOption {
	infos := assistant.Providers()
	out := make([]ProviderOption, 0, len(infos))
	for _, p := range infos {
		out = append(out, ProviderOption{ID: string(p.ID), Label: p.Label, DefaultModel: p.DefaultModel, Models: modelOptions(p.Models)})
	}
	return out
}

func defaultSettingsView() SettingsView {
	return SettingsView{
		Provider: string(assistant.DefaultProvider), Model: assistant.DefaultProvider.DefaultModel(), DailyLimit: assistant.DefaultDailyLimit,
		Enabled: true, Providers: providerOptions(),
	}
}

func toSettingsView(s assistant.Settings) SettingsView {
	return SettingsView{
		Configured: s.EncryptedKey != "", Enabled: s.Enabled, Provider: string(s.Provider), Model: s.Model,
		KeyHint: s.KeyHint, DailyLimit: s.DailyLimit, UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339),
		Providers: providerOptions(),
	}
}

func modelOptions(in []assistant.ModelInfo) []ModelOption {
	out := make([]ModelOption, 0, len(in))
	for _, m := range in {
		out = append(out, ModelOption{ID: m.ID, Free: m.Free})
	}
	return out
}
