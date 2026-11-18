package httpadapter

import (
	"io"
	"net/http"
	"strings"

	appdoc "github.com/dua-os/dua-os-be/internal/app/docintake"
	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const maxDocBody = (1 << 20) + (64 << 10)

func (a API) docsGate(w http.ResponseWriter, r *http.Request) (*appdoc.Service, bool) {
	if a.Docs == nil {
		httpx.WriteError(w, errx.New("docs_unavailable", "document simplifier is not available", errx.ErrNotImplemented))
		return nil, false
	}
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, maxDocBody)
	return a.Docs, true
}

func (a API) listWorkspaceDocs(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.docsGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	items, err := svc.List(r.Context(), ac)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"documents": items})
}

func (a API) ingestWorkspaceDoc(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.docsGate(w, r)
	if !ok {
		return
	}
	ac, ok := a.actor(w, r)
	if !ok {
		return
	}
	var in appdoc.IngestInput
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxDocBody); err != nil {
			httpx.WriteError(w, errx.Invalid("invalid_multipart", "could not parse uploaded file"))
			return
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			httpx.WriteError(w, errx.Invalid("file_required", "uploaded file is required"))
			return
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, docintake.MaxFileBytes+1))
		if err != nil {
			httpx.WriteError(w, errx.Invalid("file_unreadable", "could not read uploaded file"))
			return
		}
		in = appdoc.IngestInput{
			BusinessDate: r.FormValue("businessDate"),
			FileName:     hdr.Filename,
			SourceKind:   r.FormValue("sourceKind"),
			Content:      string(raw),
			Locale:       r.FormValue("locale"),
		}
	} else {
		var body struct {
			BusinessDate string `json:"businessDate"`
			FileName     string `json:"fileName"`
			SourceKind   string `json:"sourceKind"`
			Content      string `json:"content"`
			Locale       string `json:"locale"`
		}
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		in = appdoc.IngestInput{
			BusinessDate: body.BusinessDate,
			FileName:     body.FileName,
			SourceKind:   body.SourceKind,
			Content:      body.Content,
			Locale:       body.Locale,
		}
	}
	view, err := svc.Ingest(r.Context(), ac, in)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, view)
}
