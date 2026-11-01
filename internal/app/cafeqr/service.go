package cafeqr

import (
	"context"
	"errors"
	"time"

	"github.com/dua-os/dua-os-be/internal/app/actor"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/finance"
	"github.com/dua-os/dua-os-be/internal/domain/rbac"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/errx"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type Service struct {
	defs    DefinitionStore
	codes   CodeRepo
	inbox   Inbox
	assign  AssignmentPort
	clock   clock.Clock
	appBase string
}

func New(defs DefinitionStore, codes CodeRepo, inbox Inbox, assign AssignmentPort, clk clock.Clock, appBase string) *Service {
	if clk == nil {
		clk = clock.System{}
	}
	return &Service{defs: defs, codes: codes, inbox: inbox, assign: assign, clock: clk, appBase: appBase}
}

type CodeView struct {
	SiteID     string `json:"siteId"`
	Label      string `json:"label"`
	Area       string `json:"area"`
	Branch     string `json:"branch,omitempty"`
	Active     bool   `json:"active"`
	Token      string `json:"token"`
	PublicPath string `json:"publicPath"`
	PublicURL  string `json:"publicUrl"`
	ImagePath  string `json:"imagePath"`
	IssuedAt   string `json:"issuedAt"`
}

type VoiceView struct {
	ID                   string `json:"id"`
	SiteID               string `json:"siteId"`
	Label                string `json:"label"`
	Area                 string `json:"area"`
	Kind                 string `json:"kind"`
	Rating               int    `json:"rating"`
	Comment              string `json:"comment"`
	ContactName          string `json:"contactName,omitempty"`
	ContactPhone         string `json:"contactPhone,omitempty"`
	Unhappy              bool   `json:"unhappy"`
	NeedsRecovery        bool   `json:"needsRecovery"`
	RecoveryID           string `json:"recoveryId,omitempty"`
	RecoveryStatus       string `json:"recoveryStatus,omitempty"`
	AssigneeUserID       string `json:"assigneeUserId,omitempty"`
	LinkedTaskID         string `json:"linkedTaskId,omitempty"`
	CompensationAmount   int64  `json:"compensationAmount,omitempty"`
	CompensationCurrency string `json:"compensationCurrency,omitempty"`
	CompensationLimit    int64  `json:"compensationLimit,omitempty"`
	AckDue               string `json:"ackDue,omitempty"`
	ResolveDue           string `json:"resolveDue,omitempty"`
	LateAck              bool   `json:"lateAck"`
	LateResolve          bool   `json:"lateResolve"`
	CreatedAt            string `json:"createdAt"`
}

type PublicView struct {
	CafeName    string `json:"cafeName"`
	Label       string `json:"label"`
	Area        string `json:"area"`
	Branch      string `json:"branch,omitempty"`
	ReviewsOpen bool   `json:"reviewsOpen"`
	AckMinutes  int    `json:"ackMinutes,omitempty"`
	AfterHours  string `json:"afterHoursMessage,omitempty"`
	Timezone    string `json:"timezone"`
}

type SubmitInput struct {
	Kind         string
	Rating       int
	Comment      string
	ContactName  string
	ContactPhone string
}

func (s *Service) ListCodes(ctx context.Context, ac actor.Context) ([]CodeView, error) {
	if err := requireCafe(ac); err != nil {
		return nil, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewCafe) {
		return nil, errx.Forbidden("forbidden", "you cannot view QR codes")
	}
	def, err := s.definition(ctx, ac.Workspace.ID, ac.Workspace)
	if err != nil {
		return nil, err
	}
	out := make([]CodeView, 0, len(def.QRSites))
	for _, site := range def.QRSites {
		code, err := s.ensure(ctx, ac.Workspace.ID, site)
		if err != nil {
			return nil, err
		}
		out = append(out, s.toCodeView(def, site, code))
	}
	return out, nil
}

func (s *Service) Rotate(ctx context.Context, ac actor.Context, siteID string) (CodeView, error) {
	if err := requireCafe(ac); err != nil {
		return CodeView{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) && !rbac.Require(ac.Scope, rbac.ManageWorkspace) {
		return CodeView{}, errx.Forbidden("forbidden", "only a general manager or floor can rotate a QR code")
	}
	sid, err := id.Parse(siteID)
	if err != nil {
		return CodeView{}, errx.Invalid("invalid_qr_site", "QR location is invalid")
	}
	def, err := s.definition(ctx, ac.Workspace.ID, ac.Workspace)
	if err != nil {
		return CodeView{}, err
	}
	site, ok := findSite(def, sid)
	if !ok {
		return CodeView{}, errx.NotFound("qr_site_not_found", "QR location not found")
	}
	now := s.clock.Now().UTC()
	tok, err := cafe.NewToken()
	if err != nil {
		return CodeView{}, err
	}
	code := cafe.Code{WorkspaceID: ac.Workspace.ID, SiteID: sid, Token: tok, IssuedAt: now, RotatedAt: now}
	if err := code.Validate(); err != nil {
		return CodeView{}, err
	}
	if err := s.codes.SaveQRCode(ctx, code); err != nil {
		return CodeView{}, err
	}
	return s.toCodeView(def, site, code), nil
}

func (s *Service) Resolve(ctx context.Context, token string) (PublicView, error) {
	tok, err := cafe.ParseToken(token)
	if err != nil {
		return PublicView{}, err
	}
	code, err := s.codes.GetQRCodeByToken(ctx, tok)
	if err != nil {
		return PublicView{}, errx.NotFound("qr_not_found", "this QR code is not recognised")
	}
	def, err := s.definition(ctx, code.WorkspaceID, tenancy.Workspace{ID: code.WorkspaceID})
	if err != nil {
		return PublicView{}, err
	}
	site, ok := findSite(def, code.SiteID)
	if !ok {
		return PublicView{}, errx.NotFound("qr_not_found", "this QR code is not recognised")
	}
	if !site.Active {
		return PublicView{}, errx.Invalid("qr_inactive", "this QR location is retired")
	}
	return PublicView{
		CafeName:    def.DisplayName,
		Label:       site.Label,
		Area:        string(site.Area),
		Branch:      branchName(def, site.BranchID),
		ReviewsOpen: def.Completeness().ReadyForReviews,
		AckMinutes:  def.SLA.AckMinutes,
		AfterHours:  def.SLA.AfterHoursMessage,
		Timezone:    def.Timezone,
	}, nil
}

func (s *Service) Submit(ctx context.Context, token string, in SubmitInput) (VoiceView, error) {
	pub, err := s.Resolve(ctx, token)
	if err != nil {
		return VoiceView{}, err
	}
	if !pub.ReviewsOpen {
		return VoiceView{}, errx.Invalid("reviews_not_ready", "reviews stay closed until the cafe workshop is complete")
	}
	kind, err := cafe.ParseVoiceKind(in.Kind)
	if err != nil {
		return VoiceView{}, err
	}
	tok, _ := cafe.ParseToken(token)
	code, err := s.codes.GetQRCodeByToken(ctx, tok)
	if err != nil {
		return VoiceView{}, errx.NotFound("qr_not_found", "this QR code is not recognised")
	}
	def, err := s.definition(ctx, code.WorkspaceID, tenancy.Workspace{ID: code.WorkspaceID})
	if err != nil {
		return VoiceView{}, err
	}
	site, _ := findSite(def, code.SiteID)
	voice := cafe.Voice{
		ID:           id.New(),
		WorkspaceID:  code.WorkspaceID,
		SiteID:       code.SiteID,
		Token:        tok,
		Kind:         kind,
		Rating:       in.Rating,
		Comment:      in.Comment,
		ContactName:  in.ContactName,
		ContactPhone: in.ContactPhone,
		CreatedAt:    s.clock.Now().UTC(),
	}
	voice.Normalize()
	if err := voice.Validate(); err != nil {
		return VoiceView{}, err
	}
	if err := s.inbox.CreateVoice(ctx, voice); err != nil {
		return VoiceView{}, err
	}
	rec, _ := s.ensureRecovery(ctx, voice)
	return toVoiceView(voice, site, rec, def.SLA, s.clock.Now().UTC()), nil
}

func (s *Service) ListVoices(ctx context.Context, ac actor.Context) ([]VoiceView, error) {
	if err := requireCafe(ac); err != nil {
		return nil, err
	}
	if !rbac.Require(ac.Scope, rbac.ViewReviews) {
		return nil, errx.Forbidden("forbidden", "you cannot view customer voice")
	}
	def, err := s.definition(ctx, ac.Workspace.ID, ac.Workspace)
	if err != nil {
		return nil, err
	}
	list, err := s.inbox.ListVoices(ctx, ac.Workspace.ID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	out := make([]VoiceView, 0, len(list))
	for _, v := range list {
		site, _ := findSite(def, v.SiteID)
		rec, _ := s.ensureRecovery(ctx, v)
		out = append(out, toVoiceView(v, site, rec, def.SLA, now))
	}
	return out, nil
}

func (s *Service) EnsureWorkspace(ctx context.Context, workspaceID id.ID, name, timezone string) error {
	def, err := s.definition(ctx, workspaceID, tenancy.Workspace{ID: workspaceID, Name: name, Timezone: timezone})
	if err != nil {
		return err
	}
	for _, site := range def.QRSites {
		if _, err := s.ensure(ctx, workspaceID, site); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ImageTarget(ctx context.Context, token string) (string, error) {
	if _, err := s.Resolve(ctx, token); err != nil {
		return "", err
	}
	tok, err := cafe.ParseToken(token)
	if err != nil {
		return "", err
	}
	return cafe.PublicURL(s.appBase, tok), nil
}

func (s *Service) HasWiredSource(ctx context.Context, workspaceID id.ID) (bool, error) {
	codes, err := s.codes.ListQRCodes(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	if len(codes) == 0 {
		return false, nil
	}
	def, err := s.defs.GetCafeDefinition(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	for _, c := range codes {
		if site, ok := findSite(def, c.SiteID); ok && site.Active {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) OpenRecoveries(ctx context.Context, workspaceID id.ID, now time.Time) (int, int, error) {
	recs, err := s.inbox.ListRecoveries(ctx, workspaceID)
	if err != nil {
		return 0, 0, err
	}
	if len(recs) == 0 {
		return 0, 0, nil
	}
	voices, err := s.inbox.ListVoices(ctx, workspaceID)
	if err != nil {
		return 0, 0, err
	}
	byVoice := make(map[id.ID]cafe.Voice, len(voices))
	for _, v := range voices {
		byVoice[v.ID] = v
	}
	def, err := s.defs.GetCafeDefinition(ctx, workspaceID)
	if err != nil && !errors.Is(err, errx.ErrNotFound) {
		return 0, 0, err
	}
	open, late := 0, 0
	for _, rec := range recs {
		if rec.WorkspaceID != workspaceID || !rec.Open() {
			continue
		}
		open++
		voice := byVoice[rec.VoiceID]
		opened := rec.CreatedAt
		if !voice.CreatedAt.IsZero() {
			opened = voice.CreatedAt
		}
		clock := rec.Clock(def.SLA, opened, now)
		if clock.LateAck || clock.LateResolve {
			late++
		}
	}
	return open, late, nil
}

func (s *Service) UnhappyToday(ctx context.Context, workspaceID id.ID, now time.Time, timezone string) (int, error) {
	day := finance.BusinessDate(now, timezone)
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	start, _ := time.ParseInLocation("2006-01-02", day, loc)
	return s.inbox.CountUnhappyVoicesSince(ctx, workspaceID, start.UTC())
}

func (s *Service) ensure(ctx context.Context, wid id.ID, site cafe.QRSite) (cafe.Code, error) {
	cur, err := s.codes.GetQRCodeBySite(ctx, wid, site.ID)
	if err == nil {
		return cur, nil
	}
	if !errors.Is(err, errx.ErrNotFound) {
		return cafe.Code{}, err
	}
	tok, err := cafe.NewToken()
	if err != nil {
		return cafe.Code{}, err
	}
	code := cafe.Code{WorkspaceID: wid, SiteID: site.ID, Token: tok, IssuedAt: s.clock.Now().UTC()}
	if err := code.Validate(); err != nil {
		return cafe.Code{}, err
	}
	if err := s.codes.SaveQRCode(ctx, code); err != nil {
		return cafe.Code{}, err
	}
	return code, nil
}

func (s *Service) definition(ctx context.Context, wid id.ID, ws tenancy.Workspace) (cafe.Definition, error) {
	def, err := s.defs.GetCafeDefinition(ctx, wid)
	if err != nil {
		if errors.Is(err, errx.ErrNotFound) {
			return cafe.Seed(wid, ws.Name, ws.Timezone), nil
		}
		return cafe.Definition{}, err
	}
	return def, nil
}

func (s *Service) toCodeView(def cafe.Definition, site cafe.QRSite, code cafe.Code) CodeView {
	return CodeView{
		SiteID:     site.ID.String(),
		Label:      site.Label,
		Area:       string(site.Area),
		Branch:     branchName(def, site.BranchID),
		Active:     site.Active,
		Token:      code.Token,
		PublicPath: cafe.PublicPath(code.Token),
		PublicURL:  cafe.PublicURL(s.appBase, code.Token),
		ImagePath:  "/v1/public/qr/" + code.Token + "/image",
		IssuedAt:   code.IssuedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) Act(ctx context.Context, ac actor.Context, voiceID, action, assigneeUserID, note string, amount int64) (VoiceView, error) {
	if err := requireCafe(ac); err != nil {
		return VoiceView{}, err
	}
	if !rbac.Require(ac.Scope, rbac.ManageReviews) {
		return VoiceView{}, errx.Forbidden("forbidden", "only a general manager or floor can act on guest voice")
	}
	parsed, err := cafe.ParseRecoveryAction(action)
	if err != nil {
		return VoiceView{}, err
	}
	vid, err := id.Parse(voiceID)
	if err != nil {
		return VoiceView{}, errx.Invalid("invalid_voice", "voice is invalid")
	}
	voice, err := s.inbox.GetVoice(ctx, vid)
	if err != nil {
		return VoiceView{}, err
	}
	if voice.WorkspaceID != ac.Workspace.ID {
		return VoiceView{}, errx.NotFound("voice_not_found", "guest voice not found")
	}
	if !voice.NeedsRecovery() {
		return VoiceView{}, errx.Invalid("recovery_not_needed", "this note does not open a recovery file")
	}
	def, err := s.definition(ctx, ac.Workspace.ID, ac.Workspace)
	if err != nil {
		return VoiceView{}, err
	}
	rec, err := s.ensureRecovery(ctx, voice)
	if err != nil {
		return VoiceView{}, err
	}
	now := s.clock.Now().UTC()
	switch parsed {
	case cafe.ActionAssign:
		assignee, err := id.Parse(assigneeUserID)
		if err != nil {
			return VoiceView{}, errx.Invalid("invalid_assignee", "assignee is required")
		}
		var taskID id.ID
		if s.assign != nil {
			title := recoveryTitle(voice)
			if tid, aerr := s.assign.Assign(ctx, ac, title, voice.Comment, assigneeUserID, now, now.Add(2*time.Hour)); aerr == nil {
				taskID = tid
			} else if !errors.Is(aerr, errx.ErrForbidden) {
				return VoiceView{}, aerr
			}
		}
		if err := rec.Assign(assignee, taskID, now); err != nil {
			return VoiceView{}, err
		}
	case cafe.ActionCompensate:
		if err := rec.Compensate(amount, def.SLA.CompensationCurrency, def.SLA.CompensationLimit, def.SLA.CompensationCurrency, now); err != nil {
			return VoiceView{}, err
		}
	case cafe.ActionClose:
		if err := rec.Close(now, note); err != nil {
			return VoiceView{}, err
		}
	}
	if err := s.inbox.SaveRecovery(ctx, rec); err != nil {
		return VoiceView{}, err
	}
	site, _ := findSite(def, voice.SiteID)
	return toVoiceView(voice, site, rec, def.SLA, now), nil
}

func (s *Service) ensureRecovery(ctx context.Context, voice cafe.Voice) (cafe.Recovery, error) {
	cur, err := s.inbox.GetRecoveryByVoice(ctx, voice.ID)
	if err == nil {
		return cur, nil
	}
	if !errors.Is(err, errx.ErrNotFound) {
		return cafe.Recovery{}, err
	}
	if !voice.NeedsRecovery() {
		return cafe.Recovery{}, errx.NotFound("recovery_not_found", "no recovery file")
	}
	rec, err := cafe.OpenRecovery(voice.WorkspaceID, voice.ID, voice.CreatedAt)
	if err != nil {
		return cafe.Recovery{}, err
	}
	if err := s.inbox.SaveRecovery(ctx, rec); err != nil {
		return cafe.Recovery{}, err
	}
	return rec, nil
}

func toVoiceView(v cafe.Voice, site cafe.QRSite, rec cafe.Recovery, sla cafe.SLA, now time.Time) VoiceView {
	view := VoiceView{
		ID:            v.ID.String(),
		SiteID:        v.SiteID.String(),
		Label:         site.Label,
		Area:          string(site.Area),
		Kind:          string(v.Kind),
		Rating:        v.Rating,
		Comment:       v.Comment,
		ContactName:   v.ContactName,
		ContactPhone:  v.ContactPhone,
		Unhappy:       v.Unhappy(),
		NeedsRecovery: v.NeedsRecovery(),
		CreatedAt:     v.CreatedAt.UTC().Format(time.RFC3339),
	}
	if sla.CompensationLimit > 0 {
		view.CompensationLimit = sla.CompensationLimit
	}
	if rec.ID == id.Zero() {
		return view
	}
	clock := rec.Clock(sla, rec.CreatedAt, now)
	view.RecoveryID = rec.ID.String()
	view.RecoveryStatus = rec.Status
	view.LateAck = clock.LateAck
	view.LateResolve = clock.LateResolve
	if rec.AssigneeUserID != id.Zero() {
		view.AssigneeUserID = rec.AssigneeUserID.String()
	}
	if rec.LinkedTaskID != id.Zero() {
		view.LinkedTaskID = rec.LinkedTaskID.String()
	}
	if rec.CompensationAmount > 0 {
		view.CompensationAmount = rec.CompensationAmount
		view.CompensationCurrency = rec.CompensationCurrency
	}
	if !clock.AckDue.IsZero() {
		view.AckDue = clock.AckDue.UTC().Format(time.RFC3339)
	}
	if !clock.ResolveDue.IsZero() {
		view.ResolveDue = clock.ResolveDue.UTC().Format(time.RFC3339)
	}
	return view
}

func recoveryTitle(v cafe.Voice) string {
	switch v.Kind {
	case cafe.VoiceComplaint:
		return "Follow up: guest complaint"
	case cafe.VoiceRequest:
		return "Follow up: guest request"
	default:
		return "Follow up: low guest score"
	}
}

func findSite(def cafe.Definition, sid id.ID) (cafe.QRSite, bool) {
	for _, s := range def.QRSites {
		if s.ID == sid {
			return s, true
		}
	}
	return cafe.QRSite{}, false
}

func branchName(def cafe.Definition, bid id.ID) string {
	for _, b := range def.Branches {
		if b.ID == bid {
			return b.Name
		}
	}
	return ""
}

func requireCafe(ac actor.Context) error {
	if ac.Workspace.Kind != tenancy.KindCafe {
		return errx.Invalid("cafe_required", "QR codes are only available on a cafe workspace")
	}
	return nil
}

func (s *Service) AppBase() string { return s.appBase }
