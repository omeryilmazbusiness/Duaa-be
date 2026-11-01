package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errx.Invalid("invalid_json", "invalid request body")
	}
	return nil
}

func WriteError(w http.ResponseWriter, err error) {
	status, code, message := http.StatusInternalServerError, "internal", "internal error"
	switch {
	case errors.Is(err, errx.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "not found"
	case errors.Is(err, errx.ErrUnauthenticated):
		status, code, message = http.StatusUnauthorized, "unauthenticated", "unauthenticated"
	case errors.Is(err, errx.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "forbidden"
	case errors.Is(err, errx.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", "conflict"
	case errors.Is(err, errx.ErrInvalid):
		status, code, message = http.StatusUnprocessableEntity, "invalid", "invalid request"
	case errors.Is(err, errx.ErrNotImplemented):
		status, code, message = http.StatusNotImplemented, "not_implemented", "not implemented"
	case errors.Is(err, errx.ErrRateLimited):
		status, code, message = http.StatusTooManyRequests, "rate_limited", "too many requests"
	case errors.Is(err, errx.ErrUpstream):
		status, code, message = http.StatusBadGateway, "upstream_failure", "upstream service failed"
	}
	var coded *errx.Error
	if errors.As(err, &coded) {
		if coded.Code != "" {
			code = coded.Code
		}
		if coded.Message != "" {
			message = coded.Message
		}
	}
	body := errorBody{}
	body.Error.Code = code
	body.Error.Message = message
	WriteJSON(w, status, body)
}
