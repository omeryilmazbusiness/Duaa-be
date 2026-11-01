package assistant

import (
	"strings"
	"unicode/utf8"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const (
	MaxMessageRunes = 2000
	MaxHistory      = 4
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

// NormalizeConversation validates the client-supplied transcript. The client
// owns history (the server stays stateless) so nothing in it is trusted: roles
// are whitelisted, sizes capped, and only the most recent turns are kept.
func NormalizeConversation(in []Message) ([]Message, error) {
	out := make([]Message, 0, len(in))
	for _, m := range in {
		if m.Role != RoleUser && m.Role != RoleAssistant {
			return nil, errx.Invalid("ai_message_invalid", "message role must be user or assistant")
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if utf8.RuneCountInString(content) > MaxMessageRunes {
			return nil, errx.Invalid("ai_message_too_long", "message is too long")
		}
		out = append(out, Message{Role: m.Role, Content: content})
	}
	if len(out) == 0 || out[len(out)-1].Role != RoleUser {
		return nil, errx.Invalid("ai_message_required", "the last message must be a user question")
	}
	if len(out) > MaxHistory {
		out = out[len(out)-MaxHistory:]
	}
	for len(out) > 0 && out[0].Role != RoleUser {
		out = out[1:]
	}
	if len(out) == 0 || out[len(out)-1].Role != RoleUser {
		for i := len(in) - 1; i >= 0; i-- {
			if in[i].Role == RoleUser {
				content := strings.TrimSpace(in[i].Content)
				if content != "" {
					return []Message{{Role: RoleUser, Content: content}}, nil
				}
			}
		}
		return nil, errx.Invalid("ai_message_required", "the last message must be a user question")
	}
	return out, nil
}
