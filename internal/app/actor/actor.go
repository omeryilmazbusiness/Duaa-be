package actor

import (
	"context"

	"github.com/dua-os/dua-os-be/internal/domain/staffscope"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/ctxkey"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// Context is the authenticated actor inside one workspace.
// Shared by staff, task and brief so those modules do not import each other (DIP).
type Context struct {
	Principal ctxkey.Principal
	Workspace tenancy.Workspace
	Catalog   tenancy.Catalog
	Scope     staffscope.Kind
	Actor     staffscope.Actor
}

type WorkspaceReader interface {
	GetWorkspace(ctx context.Context, id id.ID) (tenancy.Workspace, error)
}

type Loader struct {
	spaces   WorkspaceReader
	registry *tenancy.Registry
}

func NewLoader(spaces WorkspaceReader, registry *tenancy.Registry) *Loader {
	return &Loader{spaces: spaces, registry: registry}
}

func (l *Loader) Load(ctx context.Context, p ctxkey.Principal, workspaceID string) (Context, error) {
	if p.WorkspaceID == "" || p.WorkspaceID != workspaceID {
		return Context{}, errx.Forbidden("workspace_mismatch", "token is not bound to this workspace")
	}
	wid, err := id.Parse(workspaceID)
	if err != nil {
		return Context{}, errx.Invalid("invalid_workspace", "invalid workspace id")
	}
	ws, err := l.spaces.GetWorkspace(ctx, wid)
	if err != nil {
		return Context{}, err
	}
	cat, err := l.registry.Catalog(ws.Kind)
	if err != nil {
		return Context{}, err
	}
	actor := staffscope.Actor{Role: p.Role, Department: p.Department}
	return Context{
		Principal: p,
		Workspace: ws,
		Catalog:   cat,
		Scope:     staffscope.Resolve(actor, cat),
		Actor:     actor,
	}, nil
}
