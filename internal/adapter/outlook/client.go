// Package outlook talks to Microsoft Graph for mailbox OAuth, list and send.
package outlook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	appinbox "github.com/dua-os/dua-os-be/internal/app/inbox"
	"github.com/dua-os/dua-os-be/internal/domain/inbox"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
)

const (
	maxGraphBytes = 2 << 20
	graphBase     = "https://graph.microsoft.com/v1.0"
	authBase      = "https://login.microsoftonline.com"
	mailScopes    = "offline_access User.Read Mail.Read Mail.Send"
)

type Client struct {
	HTTP         *http.Client
	ClientID     string
	ClientSecret string
	Tenant       string
	RedirectURL  string
}

func New(clientID, clientSecret, tenant, redirectURL string) *Client {
	if strings.TrimSpace(tenant) == "" {
		tenant = "common"
	}
	return &Client{
		HTTP: &http.Client{
			Timeout: 25 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 20 * time.Second,
			},
		},
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		Tenant:       strings.TrimSpace(tenant),
		RedirectURL:  strings.TrimSpace(redirectURL),
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

func (c *Client) Redirect() string {
	if c == nil {
		return ""
	}
	return c.RedirectURL
}

func (c *Client) AuthURL(state string) string {
	q := url.Values{}
	q.Set("client_id", c.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("response_mode", "query")
	q.Set("scope", mailScopes)
	q.Set("state", state)
	return fmt.Sprintf("%s/%s/oauth2/v2.0/authorize?%s", authBase, url.PathEscape(c.Tenant), q.Encode())
}

func (c *Client) Exchange(ctx context.Context, code string) (appinbox.TokenPair, appinbox.Profile, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", c.RedirectURL)
	form.Set("scope", mailScopes)
	tokens, err := c.token(ctx, form)
	if err != nil {
		return appinbox.TokenPair{}, appinbox.Profile{}, err
	}
	profile, err := c.me(ctx, tokens.AccessToken)
	if err != nil {
		return appinbox.TokenPair{}, appinbox.Profile{}, err
	}
	return tokens, profile, nil
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (appinbox.TokenPair, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("scope", mailScopes)
	return c.token(ctx, form)
}

func (c *Client) ListInbox(ctx context.Context, accessToken string) ([]inbox.RemoteMessage, error) {
	endpoint := graphBase + "/me/mailFolders/inbox/messages?$top=50&$orderby=receivedDateTime%20desc&$select=id,conversationId,subject,bodyPreview,body,from,toRecipients,receivedDateTime,isRead,hasAttachments"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, errx.Upstream("outlook_unreachable", "could not reach Microsoft Graph")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxGraphBytes))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, errx.Upstream("outlook_list_failed", "Microsoft Graph refused to list mail")
	}
	var parsed struct {
		Value []graphMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, errx.Upstream("outlook_list_failed", "Microsoft Graph returned unreadable mail")
	}
	out := make([]inbox.RemoteMessage, 0, len(parsed.Value))
	for _, item := range parsed.Value {
		out = append(out, item.toRemote())
	}
	return out, nil
}

func (c *Client) Send(ctx context.Context, accessToken string, draft inbox.Draft) error {
	recipients := make([]map[string]any, 0, len(draft.To))
	for _, addr := range draft.To {
		recipients = append(recipients, map[string]any{
			"emailAddress": map[string]string{"address": addr},
		})
	}
	payload := map[string]any{
		"message": map[string]any{
			"subject": draft.Subject,
			"body": map[string]string{
				"contentType": "Text",
				"content":     draft.Body,
			},
			"toRecipients": recipients,
		},
		"saveToSentItems": true,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphBase+"/me/sendMail", strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return errx.Upstream("outlook_unreachable", "could not reach Microsoft Graph")
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxGraphBytes))
	if res.StatusCode >= 300 {
		return errx.Upstream("outlook_send_failed", "Microsoft Graph refused to send the mail")
	}
	return nil
}

func (c *Client) token(ctx context.Context, form url.Values) (appinbox.TokenPair, error) {
	endpoint := fmt.Sprintf("%s/%s/oauth2/v2.0/token", authBase, url.PathEscape(c.Tenant))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return appinbox.TokenPair{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return appinbox.TokenPair{}, errx.Upstream("outlook_unreachable", "could not reach Microsoft login")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxGraphBytes))
	if err != nil {
		return appinbox.TokenPair{}, err
	}
	if res.StatusCode >= 300 {
		return appinbox.TokenPair{}, errx.Upstream("outlook_token_failed", "Microsoft login refused the Outlook connection")
	}
	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.AccessToken == "" {
		return appinbox.TokenPair{}, errx.Upstream("outlook_token_failed", "Microsoft login returned an unreadable token")
	}
	exp := time.Now().UTC().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	if parsed.ExpiresIn <= 0 {
		exp = time.Now().UTC().Add(55 * time.Minute)
	}
	return appinbox.TokenPair{
		AccessToken: parsed.AccessToken, RefreshToken: parsed.RefreshToken,
		ExpiresAt: exp, Scopes: parsed.Scope,
	}, nil
}

func (c *Client) me(ctx context.Context, accessToken string) (appinbox.Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, graphBase+"/me?$select=displayName,mail,userPrincipalName", nil)
	if err != nil {
		return appinbox.Profile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return appinbox.Profile{}, errx.Upstream("outlook_unreachable", "could not reach Microsoft Graph")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxGraphBytes))
	if err != nil {
		return appinbox.Profile{}, err
	}
	if res.StatusCode >= 300 {
		return appinbox.Profile{}, errx.Upstream("outlook_profile_failed", "could not read the Outlook profile")
	}
	var parsed struct {
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return appinbox.Profile{}, errx.Upstream("outlook_profile_failed", "could not read the Outlook profile")
	}
	email := strings.TrimSpace(parsed.Mail)
	if email == "" {
		email = strings.TrimSpace(parsed.UserPrincipalName)
	}
	return appinbox.Profile{Email: email, DisplayName: strings.TrimSpace(parsed.DisplayName)}, nil
}

type graphMessage struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	Subject        string `json:"subject"`
	BodyPreview    string `json:"bodyPreview"`
	Body           struct {
		ContentType string `json:"contentType"`
		Content     string `json:"content"`
	} `json:"body"`
	From struct {
		EmailAddress struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"emailAddress"`
	} `json:"from"`
	ToRecipients []struct {
		EmailAddress struct {
			Address string `json:"address"`
		} `json:"emailAddress"`
	} `json:"toRecipients"`
	ReceivedDateTime string `json:"receivedDateTime"`
	IsRead           bool   `json:"isRead"`
	HasAttachments   bool   `json:"hasAttachments"`
}

func (g graphMessage) toRemote() inbox.RemoteMessage {
	tos := make([]string, 0, len(g.ToRecipients))
	for _, r := range g.ToRecipients {
		if a := strings.TrimSpace(r.EmailAddress.Address); a != "" {
			tos = append(tos, a)
		}
	}
	body := strings.TrimSpace(g.Body.Content)
	if body == "" {
		body = strings.TrimSpace(g.BodyPreview)
	}
	received, err := time.Parse(time.RFC3339, g.ReceivedDateTime)
	if err != nil {
		received = time.Now().UTC()
	}
	return inbox.RemoteMessage{
		ExternalID: g.ID, ConversationID: g.ConversationID, Folder: inbox.FolderInbox,
		FromName: g.From.EmailAddress.Name, FromEmail: g.From.EmailAddress.Address,
		ToEmails: strings.Join(tos, ", "), Subject: g.Subject, BodyText: stripTags(body),
		ReceivedAt: received.UTC(), IsRead: g.IsRead, HasAttachments: g.HasAttachments,
	}
}

func stripTags(s string) string {
	if !strings.Contains(s, "<") {
		return s
	}
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

var _ appinbox.MailClient = (*Client)(nil)
