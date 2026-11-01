package httpadapter

import (
	"net/http"
	"strings"

	"github.com/dua-os/dua-os-be/internal/platform/authn"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/google/uuid"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-Id")
		if rid == "" {
			rid = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", rid)
		next.ServeHTTP(w, r.WithContext(ctxkey.WithRequestID(r.Context(), rid)))
	})
}

func Authenticate(tokens *authn.JWT) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httpx.WriteError(w, errx.Unauthenticated("missing bearer token"))
				return
			}
			p, err := tokens.Parse(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				httpx.WriteError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctxkey.WithPrincipal(r.Context(), p)))
		})
	}
}

func Principal(r *http.Request) (ctxkey.Principal, error) {
	p, ok := ctxkey.PrincipalFrom(r.Context())
	if !ok {
		return ctxkey.Principal{}, errx.Unauthenticated("unauthenticated")
	}
	return p, nil
}
