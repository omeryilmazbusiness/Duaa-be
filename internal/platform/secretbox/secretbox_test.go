package secretbox_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dua-os/dua-os-be/internal/platform/secretbox"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestRoundTripAndBinding(t *testing.T) {
	box, err := secretbox.New(secret)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("sk-live-secret", "ws-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "sk-live-secret") {
		t.Fatal("ciphertext must not contain plaintext")
	}
	again, _ := box.Seal("sk-live-secret", "ws-1")
	if again == sealed {
		t.Fatal("nonce must make every seal unique")
	}
	plain, err := box.Open(sealed, "ws-1")
	if err != nil || plain != "sk-live-secret" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	if _, err := box.Open(sealed, "ws-2"); !errors.Is(err, secretbox.ErrOpen) {
		t.Fatalf("other owner must not open: %v", err)
	}
}

func TestRejectsTamperAndWrongKey(t *testing.T) {
	box, _ := secretbox.New(secret)
	other, _ := secretbox.New("ffffffffffffffffffffffffffffffff")
	sealed, _ := box.Seal("value", "a")
	if _, err := other.Open(sealed, "a"); err == nil {
		t.Fatal("wrong key must not open")
	}
	if _, err := box.Open(sealed[:len(sealed)-2]+"AA", "a"); err == nil {
		t.Fatal("tampered ciphertext must not open")
	}
	if _, err := box.Open("garbage", "a"); err == nil {
		t.Fatal("garbage must not open")
	}
	if _, err := secretbox.New("short"); err == nil {
		t.Fatal("short secret must be rejected")
	}
}
