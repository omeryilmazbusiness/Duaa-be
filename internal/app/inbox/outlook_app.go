package inbox

import (
	"context"
	"strings"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

// SaveOutlookApp stores the workspace Azure app (client id/secret) used for real Outlook OAuth.
func (s *Service) SaveOutlookApp(ctx context.Context, ac actor.Context, clientID, clientSecret, tenant string) (StatusView, error) {
	if err := requireInbox(ac); err != nil {
		return StatusView{}, err
	}
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		tenant = "common"
	}
	if clientID == "" || clientSecret == "" {
		return StatusView{}, errx.Invalid("missing_outlook_app", "Azure application client id and secret are required")
	}
	if s.factory == nil {
		return StatusView{}, errx.Upstream("outlook_not_configured", "Outlook client factory is not wired")
	}
	uid, err := id.Parse(ac.Principal.UserID)
	if err != nil {
		return StatusView{}, errx.Unauthenticated("invalid token")
	}
	sealed, err := s.cipher.Seal(clientSecret, ac.Workspace.ID.String())
	if err != nil {
		return StatusView{}, err
	}
	now := s.clock.Now().UTC()
	conn, err := s.repo.GetConnection(ctx, ac.Workspace.ID)
	if err != nil {
		conn = inbox.Connection{
			WorkspaceID: ac.Workspace.ID, Provider: inbox.ProviderOutlook,
			LastSyncStatus: inbox.SyncNever, ConnectedBy: uid, CreatedAt: now,
		}
	}
	conn.ClientID = clientID
	conn.EncryptedClientSecret = sealed
	conn.Tenant = tenant
	conn.Provider = inbox.ProviderOutlook
	conn.ConnectedBy = uid
	conn.UpdatedAt = now
	if err := s.repo.SaveConnection(ctx, conn); err != nil {
		return StatusView{}, err
	}
	return s.Status(ctx, ac)
}

func (s *Service) mailClient(ctx context.Context, workspaceID id.ID) (MailClient, *inbox.Connection, error) {
	conn, err := s.repo.GetConnection(ctx, workspaceID)
	if err == nil && conn.ClientID != "" && conn.EncryptedClientSecret != "" && s.factory != nil {
		secret, oerr := s.cipher.Open(conn.EncryptedClientSecret, workspaceID.String())
		if oerr != nil {
			return nil, &conn, errx.Upstream("outlook_app_unreadable", "the stored Outlook app secret could not be opened; save it again")
		}
		tenant := conn.Tenant
		if tenant == "" {
			tenant = "common"
		}
		client := s.factory(conn.ClientID, secret, tenant)
		if client == nil || !client.Configured() {
			return nil, &conn, errx.Upstream("outlook_not_configured", "Outlook app credentials are incomplete")
		}
		return client, &conn, nil
	}
	if s.outlook != nil && s.outlook.Configured() {
		var cptr *inbox.Connection
		if err == nil {
			cptr = &conn
		}
		return s.outlook, cptr, nil
	}
	return nil, nil, errx.Upstream("outlook_not_configured", "Save your Azure app client id and secret, or set MS_GRAPH_CLIENT_ID on the server")
}

func (s *Service) redirectURLHint() string {
	if s.outlook != nil {
		// best-effort: factory clients share the same redirect as the server client
		if c, ok := s.outlook.(interface{ Redirect() string }); ok {
			return c.Redirect()
		}
	}
	return strings.TrimRight(s.public, "/") + "/v1/inbox/outlook/callback"
}
