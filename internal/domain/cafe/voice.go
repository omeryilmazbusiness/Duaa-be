package cafe

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const (
	MinRating     = 1
	MaxRating     = 5
	MaxComment    = 500
	MaxContact    = 80
	MaxPhone      = 32
	MinAskComment = 2
	UnhappyRating = 3
)

type VoiceKind string

const (
	VoiceReview    VoiceKind = "review"
	VoiceComplaint VoiceKind = "complaint"
	VoiceRequest   VoiceKind = "request"
)

// Voice is one guest scan. Recovery (assign, compensate) is a later vertical.
type Voice struct {
	ID           id.ID
	WorkspaceID  id.ID
	SiteID       id.ID
	Token        string
	Kind         VoiceKind
	Rating       int
	Comment      string
	ContactName  string
	ContactPhone string
	CreatedAt    time.Time
}

func ParseVoiceKind(raw string) (VoiceKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(VoiceReview):
		return VoiceReview, nil
	case string(VoiceComplaint):
		return VoiceComplaint, nil
	case string(VoiceRequest):
		return VoiceRequest, nil
	default:
		return "", errx.Invalid("invalid_voice_kind", "voice must be review, complaint or request")
	}
}

func (v *Voice) Normalize() {
	if v == nil {
		return
	}
	if kind, err := ParseVoiceKind(string(v.Kind)); err == nil {
		v.Kind = kind
	}
	v.Comment = clip(strings.TrimSpace(v.Comment), MaxComment)
	v.ContactName = clip(strings.TrimSpace(v.ContactName), MaxContact)
	v.ContactPhone = clip(strings.TrimSpace(v.ContactPhone), MaxPhone)
}

func (v Voice) Validate() error {
	if v.WorkspaceID == id.Zero() || v.SiteID == id.Zero() {
		return errx.Invalid("invalid_voice", "workspace and site are required")
	}
	kind, err := ParseVoiceKind(string(v.Kind))
	if err != nil {
		return err
	}
	if v.Rating != 0 && (v.Rating < MinRating || v.Rating > MaxRating) {
		return errx.Invalid("invalid_rating", "rating must be between 1 and 5")
	}
	if kind == VoiceReview && (v.Rating < MinRating || v.Rating > MaxRating) {
		return errx.Invalid("invalid_rating", "rating must be between 1 and 5")
	}
	if (kind == VoiceComplaint || kind == VoiceRequest) && utf8.RuneCountInString(v.Comment) < MinAskComment {
		return errx.Invalid("invalid_comment", "say what happened")
	}
	if utf8.RuneCountInString(v.Comment) > MaxComment {
		return errx.Invalid("invalid_comment", "comment is too long")
	}
	return nil
}

func (v Voice) Unhappy() bool {
	return v.Kind == VoiceComplaint || (v.Rating > 0 && v.Rating <= UnhappyRating)
}
