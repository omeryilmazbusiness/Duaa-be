package authn

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

const issuer = "dua-os"

type Claims struct {
	UserID       string `json:"user_id"`
	WorkspaceID  string `json:"workspace_id"`
	MembershipID string `json:"membership_id"`
	Role         string `json:"role"`
	Department   string `json:"department"`
	Scope        string `json:"scope"`
	jwt.RegisteredClaims
}

type JWT struct {
	secret []byte
	ttl    time.Duration
}

func NewJWT(secret string, ttl time.Duration) (*JWT, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("jwt secret must be at least 32 characters")
	}
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &JWT{secret: []byte(secret), ttl: ttl}, nil
}

func (j *JWT) Issue(p ctxkey.Principal, now time.Time) (string, time.Duration, error) {
	claims := Claims{
		UserID:       p.UserID,
		WorkspaceID:  p.WorkspaceID,
		MembershipID: p.MembershipID,
		Role:         p.Role,
		Department:   p.Department,
		Scope:        p.Scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   p.UserID,
			ID:        id.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(j.secret)
	if err != nil {
		return "", 0, err
	}
	return signed, j.ttl, nil
}

func (j *JWT) Parse(raw string) (ctxkey.Principal, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errx.Unauthenticated("unexpected signing method")
		}
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return ctxkey.Principal{}, errx.Unauthenticated("invalid token")
	}
	return ctxkey.Principal{
		UserID:       claims.UserID,
		WorkspaceID:  claims.WorkspaceID,
		MembershipID: claims.MembershipID,
		Role:         claims.Role,
		Department:   claims.Department,
		Scope:        claims.Scope,
	}, nil
}
