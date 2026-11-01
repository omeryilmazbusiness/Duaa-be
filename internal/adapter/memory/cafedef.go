package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dua-os/dua-os-be/internal/adapter/cafedoc"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type persistedCafe struct {
	Definitions []cafedoc.Document `json:"definitions"`
}

func (s *Store) AttachCafePersist(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cafePersist = path
	return s.loadCafeLocked()
}

func (s *Store) GetCafeDefinition(_ context.Context, workspaceID id.ID) (cafe.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.cafeDefs[workspaceID]
	if !ok {
		return cafe.Definition{}, errx.NotFound("cafe_definition_not_found", "cafe definition not found")
	}
	return d.Clone(), nil
}

func (s *Store) SaveCafeDefinition(_ context.Context, def cafe.Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cafeDefs == nil {
		s.cafeDefs = map[id.ID]cafe.Definition{}
	}
	s.cafeDefs[def.WorkspaceID] = def.Clone()
	return s.flushCafeLocked()
}

func (s *Store) loadCafeLocked() error {
	if s.cafePersist == "" {
		return nil
	}
	raw, err := os.ReadFile(s.cafePersist)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file persistedCafe
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	if s.cafeDefs == nil {
		s.cafeDefs = map[id.ID]cafe.Definition{}
	}
	for _, doc := range file.Definitions {
		d, err := cafedoc.ToDomain(doc)
		if err != nil {
			continue
		}
		s.cafeDefs[d.WorkspaceID] = d
	}
	return nil
}

func (s *Store) flushCafeLocked() error {
	if s.cafePersist == "" {
		return nil
	}
	file := persistedCafe{Definitions: make([]cafedoc.Document, 0, len(s.cafeDefs))}
	for _, d := range s.cafeDefs {
		file.Definitions = append(file.Definitions, cafedoc.FromDomain(d))
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cafePersist), 0o700); err != nil {
		return err
	}
	tmp := s.cafePersist + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.cafePersist)
}
