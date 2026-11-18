package assistant

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	MaxTranslateRunes  = 3000
	MaxTranscriptRunes = 16000
	MaxAudioBytes      = 12 << 20 // 12 MiB
	MaxSpeakRunes      = 1500
)

type TranslateMode string

const (
	ModeTranslate   TranslateMode = "translate"
	ModeExplainTerm TranslateMode = "explain_term"
)

type MeetingItem struct {
	Text      string `json:"text"`
	Owner     string `json:"owner,omitempty"`
	DueAt     string `json:"dueAt,omitempty"`
	Ambiguous bool   `json:"ambiguous"`
}

type MeetingRecord struct {
	ID                   id.ID
	WorkspaceID          id.ID
	CreatedBy            id.ID
	BusinessDate         string
	Title                string
	Participants         []string
	Transcript           string
	TranslatedTranscript string
	Summary              string
	Decisions            []MeetingItem
	ActionItems          []MeetingItem
	SourceLocale         string
	TargetLocale         string
	AIUsed               bool
	CreatedAt            time.Time
}

func ParseTranslateMode(raw string) (TranslateMode, error) {
	switch TranslateMode(strings.ToLower(strings.TrimSpace(raw))) {
	case "", ModeTranslate:
		return ModeTranslate, nil
	case ModeExplainTerm:
		return ModeExplainTerm, nil
	default:
		return "", errx.Invalid("invalid_translate_mode", "mode must be translate or explain_term")
	}
}

func NormalizeLangPair(src, dst string) (string, string, error) {
	s := normalizeLang(src, "auto")
	d := normalizeLang(dst, "ar")
	if d == "auto" {
		d = "ar"
	}
	return s, d, nil
}

func normalizeLang(raw, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "en", "english":
		return "en"
	case "ar", "arabic":
		return "ar"
	case "tr", "turkish":
		return "tr"
	case "auto", "":
		return fallback
	default:
		return fallback
	}
}

func NormalizeTranslateText(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errx.Invalid("empty_text", "text is required")
	}
	if utf8.RuneCountInString(s) > MaxTranslateRunes {
		return "", errx.Invalid("text_too_long", "text is too long to translate")
	}
	return s, nil
}

func NormalizeTranscript(title, raw string) (string, string, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		t = "Executive Meeting"
	}
	if r := []rune(t); len(r) > 120 {
		t = string(r[:120])
	}
	body := strings.TrimSpace(raw)
	if body == "" {
		return "", "", errx.Invalid("empty_transcript", "meeting transcript is required")
	}
	if utf8.RuneCountInString(body) > MaxTranscriptRunes {
		return "", "", errx.Invalid("transcript_too_long", "meeting transcript exceeds maximum size")
	}
	return t, body, nil
}

// FallbackMeetingSummary extracts deterministic lines when AI is offline, marking ambiguous owners (AT-08).
func FallbackMeetingSummary(transcript string) (summary string, decisions []MeetingItem, actions []MeetingItem) {
	lines := strings.Split(transcript, "\n")
	var kept []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if len(kept) < 4 {
			kept = append(kept, line)
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "decide") || strings.Contains(lower, "agreed") || strings.Contains(lower, "approved") || strings.Contains(lower, "karar") || strings.Contains(lower, "قرار") {
			decisions = append(decisions, MeetingItem{Text: line, Ambiguous: true})
		} else if strings.Contains(lower, "will ") || strings.Contains(lower, "action") || strings.Contains(lower, "follow") || strings.Contains(lower, "yapacak") || strings.Contains(lower, "متابعة") {
			actions = append(actions, MeetingItem{Text: line, Ambiguous: true})
		}
	}
	if len(kept) == 0 {
		summary = "Meeting recorded."
	} else {
		summary = strings.Join(kept, " · ")
	}
	return summary, decisions, actions
}
