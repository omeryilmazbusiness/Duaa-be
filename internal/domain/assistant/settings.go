// Package assistant holds the pure rules of the DUAA AI assistant: providers,
// per-workspace settings, conversation limits and the guardrail prompt.
package assistant

import (
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderGemini Provider = "gemini"
)

// DefaultProvider is preselected for a workspace that has not configured the
// assistant yet. Gemini is first because its free API tier needs no billing.
const DefaultProvider = ProviderGemini

const (
	DefaultDailyLimit = 30
	MinDailyLimit     = 1
	MaxDailyLimit     = 500

	minKeyLen = 16
	maxKeyLen = 256
)

// modelPattern keeps the model id safe to embed in a provider URL path while
// still allowing new models without a deploy.
var modelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

// ModelInfo is one selectable model. Free marks models the provider serves on
// its free API tier (rate limited, no billing needed).
type ModelInfo struct {
	ID   string
	Free bool
}

// ProviderInfo is the single source of truth the UI renders its picker from.
// Any other model id that passes NormalizeModel is accepted too, so new
// releases work without a deploy.
type ProviderInfo struct {
	ID           Provider
	Label        string
	DefaultModel string
	Models       []ModelInfo
}

// Providers lists every supported provider in display order (the default
// provider first). The Gemini default model is on the free tier so a key from
// Google AI Studio works at once.
func Providers() []ProviderInfo {
	return []ProviderInfo{
		{ID: ProviderGemini, Label: "Google Gemini", DefaultModel: "gemini-3.1-flash-lite", Models: []ModelInfo{
			{ID: "gemini-3.1-flash-lite", Free: true},
			{ID: "gemini-3.5-flash-lite", Free: true},
			{ID: "gemini-3.5-flash", Free: true},
			{ID: "gemini-3.6-flash", Free: true},
			{ID: "gemini-3.8-flash", Free: true},
			{ID: "gemini-2.5-flash-lite", Free: true},
			{ID: "gemini-2.5-flash", Free: true},
			{ID: "gemini-2.0-flash", Free: true},
		}},
		{ID: ProviderOpenAI, Label: "OpenAI (GPT)", DefaultModel: "gpt-4o-mini", Models: []ModelInfo{
			{ID: "gpt-4o-mini"}, {ID: "gpt-4o"},
			{ID: "gpt-4.1-nano"}, {ID: "gpt-4.1-mini"}, {ID: "gpt-4.1"},
		}},
	}
}

// InfoFor returns the catalogue entry of a provider.
func InfoFor(p Provider) (ProviderInfo, bool) {
	for _, info := range Providers() {
		if info.ID == p {
			return info, true
		}
	}
	return ProviderInfo{}, false
}

func ParseProvider(raw string) (Provider, error) {
	p := Provider(strings.ToLower(strings.TrimSpace(raw)))
	for _, info := range Providers() {
		if info.ID == p {
			return p, nil
		}
	}
	return "", errx.Invalid("ai_provider_invalid", "provider must be openai or gemini")
}

func (p Provider) DefaultModel() string {
	info, _ := InfoFor(p)
	return info.DefaultModel
}

// FastFallback is the generate-safe model to try when the stored id 404s or
// 400s. Empty means the current model is already that fallback.
func FastFallback(p Provider, current string) string {
	next := p.DefaultModel()
	if next == "" || next == current {
		return ""
	}
	return next
}

// NormalizeModel returns the provider default when raw is empty.
func NormalizeModel(p Provider, raw string) (string, error) {
	model := strings.TrimPrefix(strings.TrimSpace(raw), "models/")
	if model == "" {
		return p.DefaultModel(), nil
	}
	if !modelPattern.MatchString(model) {
		return "", errx.Invalid("ai_model_invalid", "model name has invalid characters")
	}
	return model, nil
}

// ResolveModel picks a model the key can actually call. Catalog aliases such
// as gemini-2.5-flash often 404; the live list has the dated/preview id.
func ResolveModel(wanted string, curated []ModelInfo, live []string) string {
	wanted = strings.TrimPrefix(strings.TrimSpace(wanted), "models/")
	if len(live) == 0 {
		return wanted
	}
	if alias := bestAlias(wanted, live); alias != "" {
		return alias
	}
	// Live lists still advertise retired 2.x ids that 404 on generate.
	// Prefer a generate-safe 3.x flash when the wanted alias is absent.
	sorted := append([]string(nil), live...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return generateRank(sorted[i]) < generateRank(sorted[j])
	})
	if len(sorted) > 0 && generateRank(sorted[0]) <= 2 {
		return sorted[0]
	}
	for _, m := range curated {
		if alias := bestAlias(m.ID, live); alias != "" {
			return alias
		}
	}
	return live[0]
}

func bestAlias(wanted string, live []string) string {
	if wanted == "" {
		return ""
	}
	var matches []string
	for _, id := range live {
		if id == wanted || strings.HasPrefix(id, wanted+"-") {
			matches = append(matches, id)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return aliasRank(wanted, matches[i]) < aliasRank(wanted, matches[j])
	})
	return matches[0]
}

func aliasRank(wanted, id string) int {
	switch {
	case id == wanted:
		return 0
	case id == wanted+"-latest":
		return 1
	case strings.Contains(id, "preview"):
		return 2
	case strings.Contains(id, "-lite"):
		return 4
	default:
		return 3
	}
}

// MaxSetupTries is how many generate calls a key setup or chat recovery may
// make. More than this blows past reverse-proxy deadlines (Next.js ≈30s).
const MaxSetupTries = 3

// SetupTries is the short list of models to ping. Catalogue aliases like
// gemini-2.5-flash often appear in the live list but 404 on generate, so
// widely-served flash ids are tried next.
func SetupTries(wanted string, curated []ModelInfo, live []string) []string {
	out := make([]string, 0, MaxSetupTries)
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimPrefix(strings.TrimSpace(id), "models/")
		if id == "" || seen[id] || len(out) >= MaxSetupTries {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(live) > 0 {
		add(ResolveModel(wanted, curated, live))
		sorted := append([]string(nil), live...)
		sort.SliceStable(sorted, func(i, j int) bool {
			return generateRank(sorted[i]) < generateRank(sorted[j])
		})
		for _, id := range sorted {
			add(id)
		}
		return out
	}
	for _, id := range VerifyCandidates(wanted, curated) {
		add(id)
	}
	return out
}

func generateRank(id string) int {
	switch {
	case id == "gemini-3.1-flash-lite" || id == "gemini-3.5-flash-lite":
		return 0
	case strings.Contains(id, "3.1-flash-lite") || strings.Contains(id, "3.5-flash-lite"):
		return 1
	case strings.Contains(id, "3.5-flash") || strings.Contains(id, "3.6-flash"):
		return 2
	case strings.Contains(id, "flash-lite"):
		return 3
	case strings.Contains(id, "flash"):
		return 4
	default:
		return 5
	}
}

// VerifyCandidates is the order to try when the provider 404s a model: the
// user's choice, then the curated list (free first).
func VerifyCandidates(wanted string, curated []ModelInfo) []string {
	out := make([]string, 0, len(curated)+1)
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimPrefix(strings.TrimSpace(id), "models/")
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(wanted)
	var paid []string
	for _, m := range curated {
		if m.Free {
			add(m.ID)
		} else {
			paid = append(paid, m.ID)
		}
	}
	for _, id := range paid {
		add(id)
	}
	return out
}

// MergeModels puts the curated models first (keeping their free-tier marks),
// then appends any other live model ids in a stable order. Invalid ids are
// dropped so they can never reach a provider URL.
func MergeModels(curated []ModelInfo, live []string) []ModelInfo {
	out := make([]ModelInfo, 0, len(curated)+len(live))
	seen := make(map[string]bool, len(curated)+len(live))
	liveSet := make(map[string]bool, len(live))
	for _, id := range live {
		liveSet[id] = true
	}
	for _, m := range curated {
		if liveSet[m.ID] {
			out = append(out, m)
			seen[m.ID] = true
		}
	}
	extra := make([]string, 0, len(live))
	for _, id := range live {
		if !seen[id] && modelPattern.MatchString(id) {
			seen[id] = true
			extra = append(extra, id)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(extra)))
	for _, id := range extra {
		out = append(out, ModelInfo{ID: id})
	}
	if len(out) == 0 {
		return curated
	}
	return out
}

func NormalizeDailyLimit(limit int) (int, error) {
	if limit < MinDailyLimit || limit > MaxDailyLimit {
		return 0, errx.Invalid("ai_limit_invalid", "daily limit must be between 1 and 500")
	}
	return limit, nil
}

// NormalizeAPIKey trims and sanity-checks a pasted key without revealing it.
func NormalizeAPIKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if len(key) < minKeyLen || len(key) > maxKeyLen {
		return "", errx.Invalid("ai_key_invalid", "API key looks invalid")
	}
	for _, r := range key {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r > unicode.MaxASCII {
			return "", errx.Invalid("ai_key_invalid", "API key looks invalid")
		}
	}
	return key, nil
}

// KeyHint is the only part of the key that is ever shown again.
func KeyHint(key string) string {
	if len(key) <= 4 {
		return ""
	}
	return key[len(key)-4:]
}

// Settings is one workspace's assistant configuration. EncryptedKey is opaque
// ciphertext and must never be serialised to a client.
type Settings struct {
	WorkspaceID     id.ID
	Provider        Provider
	Model           string
	EncryptedKey    string
	KeyHint         string
	DailyLimit      int
	Enabled         bool
	UpdatedByUserID id.ID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s Settings) Available() bool { return s.EncryptedKey != "" && s.Enabled }

// NextReset is the next local midnight, when the daily counter starts over.
func NextReset(now time.Time, timezone string) time.Time {
	loc, err := time.LoadLocation(timezone)
	if err != nil || loc == nil {
		loc = time.UTC
	}
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
}
