package inbox

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Provider string
type SyncStatus string
type Folder string
type Category string
type ComposeLang string

const (
	ProviderOutlook Provider = "outlook"
	ProviderDemo    Provider = "demo"
)

const (
	SyncNever SyncStatus = "never"
	SyncOK    SyncStatus = "ok"
	SyncError SyncStatus = "error"
)

const (
	FolderInbox Folder = "inbox"
	FolderSent  Folder = "sent"
)

const (
	CatGuest       Category = "guest"
	CatReservation Category = "reservation"
	CatSupplier    Category = "supplier"
	CatStaff       Category = "staff"
	CatFinance     Category = "finance"
	CatComplaint   Category = "complaint"
	CatMarketing   Category = "marketing"
	CatInternal    Category = "internal"
	CatOther       Category = "other"
)

const (
	ComposeEN ComposeLang = "en"
	ComposeTR ComposeLang = "tr"
)

const (
	MaxSubject     = 300
	MaxBody        = 40_000
	MaxPreview     = 280
	MaxSummary     = 1_200
	MaxRecipients  = 20
	MaxEmailLen    = 254
	MaxDisplayName = 120
)

// Connection is one Outlook (or demo) mailbox bound to a workspace.
type Connection struct {
	WorkspaceID           id.ID
	Provider              Provider
	Email                 string
	DisplayName           string
	EncryptedAccessToken  string
	EncryptedRefreshToken string
	ClientID               string
	EncryptedClientSecret  string
	Tenant                 string
	TokenExpiresAt        *time.Time
	Scopes                string
	LastSyncAt            *time.Time
	LastSyncStatus        SyncStatus
	LastError             string
	ConnectedBy           id.ID
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Message is a cached mailbox item with optional AI enrichment.
type Message struct {
	ID               id.ID
	WorkspaceID      id.ID
	ExternalID       string
	ConversationID   string
	Folder           Folder
	FromName         string
	FromEmail        string
	ToEmails         string
	Subject          string
	BodyText         string
	Preview          string
	ReceivedAt       time.Time
	IsRead           bool
	HasAttachments   bool
	Category         Category
	Summary          string
	SummaryAR        string
	BodyAR           string
	DetectedLanguage string
	EnrichedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RemoteMessage is what a mail provider returns before local persistence.
type RemoteMessage struct {
	ExternalID     string
	ConversationID string
	Folder         Folder
	FromName       string
	FromEmail      string
	ToEmails       string
	Subject        string
	BodyText       string
	ReceivedAt     time.Time
	IsRead         bool
	HasAttachments bool
}

type Draft struct {
	To      []string
	Subject string
	Body    string
}

func ParseCategory(raw string) (Category, error) {
	switch Category(strings.ToLower(strings.TrimSpace(raw))) {
	case CatGuest, CatReservation, CatSupplier, CatStaff, CatFinance, CatComplaint, CatMarketing, CatInternal, CatOther:
		return Category(strings.ToLower(strings.TrimSpace(raw))), nil
	case "":
		return "", nil
	default:
		return "", errx.Invalid("invalid_category", "unknown mail category")
	}
}

func ParseComposeLang(raw string) (ComposeLang, error) {
	switch ComposeLang(strings.ToLower(strings.TrimSpace(raw))) {
	case ComposeEN, ComposeTR:
		return ComposeLang(strings.ToLower(strings.TrimSpace(raw))), nil
	default:
		return "", errx.Invalid("invalid_compose_lang", "compose language must be en or tr")
	}
}

func NormalizeDraft(to []string, subject, body string) (Draft, error) {
	addrs := make([]string, 0, len(to))
	seen := map[string]struct{}{}
	for _, raw := range to {
		addr := strings.ToLower(strings.TrimSpace(raw))
		if addr == "" {
			continue
		}
		if len(addr) > MaxEmailLen {
			return Draft{}, errx.Invalid("invalid_recipient", "recipient email is too long")
		}
		if _, err := mail.ParseAddress(addr); err != nil {
			return Draft{}, errx.Invalid("invalid_recipient", "recipient email is invalid")
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		return Draft{}, errx.Invalid("missing_recipient", "at least one recipient is required")
	}
	if len(addrs) > MaxRecipients {
		return Draft{}, errx.Invalid("too_many_recipients", "too many recipients")
	}
	subj := strings.TrimSpace(subject)
	if subj == "" {
		return Draft{}, errx.Invalid("missing_subject", "subject is required")
	}
	if utf8.RuneCountInString(subj) > MaxSubject {
		return Draft{}, errx.Invalid("subject_too_long", "subject is too long")
	}
	text := strings.TrimSpace(body)
	if text == "" {
		return Draft{}, errx.Invalid("missing_body", "message body is required")
	}
	if utf8.RuneCountInString(text) > MaxBody {
		return Draft{}, errx.Invalid("body_too_long", "message body is too long")
	}
	return Draft{To: addrs, Subject: subj, Body: text}, nil
}

func PreviewOf(body string) string {
	clean := strings.Join(strings.Fields(body), " ")
	if utf8.RuneCountInString(clean) <= MaxPreview {
		return clean
	}
	runes := []rune(clean)
	return string(runes[:MaxPreview-1]) + "…"
}

func ConnectionNotFoundError() error {
	return errx.NotFound("inbox_not_connected", "no Outlook mailbox is connected for this business")
}

func MessageNotFoundError() error {
	return errx.NotFound("inbox_message_not_found", "that mail was not found")
}
