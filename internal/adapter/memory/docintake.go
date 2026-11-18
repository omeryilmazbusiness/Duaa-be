package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/dua-os/dua-os-be/internal/domain/docintake"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedDocs struct {
	Docs []docintake.Document `json:"docs"`
}

func (s *Store) AttachDocPersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docPersist = path
	return s.loadDocLocked()
}

func (s *Store) GetDocumentByHash(_ context.Context, workspaceID id.ID, date, sha string) (docintake.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.docs {
		if d.WorkspaceID == workspaceID && d.BusinessDate == date && d.ContentSHA256 == sha {
			return d, nil
		}
	}
	return docintake.Document{}, errx.NotFound("document_not_found", "document not found")
}

func (s *Store) SaveDocument(_ context.Context, doc docintake.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.docs == nil {
		s.docs = map[id.ID]docintake.Document{}
	}
	s.docs[doc.ID] = doc
	return s.flushDocLocked()
}

func (s *Store) ListDocuments(_ context.Context, workspaceID id.ID, limit int) ([]docintake.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]docintake.Document, 0)
	for _, d := range s.docs {
		if d.WorkspaceID == workspaceID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) flushDocLocked() error {
	if s.docPersist == "" {
		return nil
	}
	payload := persistedDocs{Docs: make([]docintake.Document, 0, len(s.docs))}
	for _, d := range s.docs {
		payload.Docs = append(payload.Docs, d)
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.docPersist), 0o700); err != nil {
		return err
	}
	tmp := s.docPersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.docPersist)
}

func (s *Store) loadDocLocked() error {
	if s.docPersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.docPersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload persistedDocs
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	s.docs = map[id.ID]docintake.Document{}
	for _, d := range payload.Docs {
		if d.ID != id.Zero() {
			s.docs[d.ID] = d
		}
	}
	return nil
}
