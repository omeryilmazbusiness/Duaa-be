package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dua-os/dua-os-be/internal/adapter/assistantctx"
	httpadapter "github.com/dua-os/dua-os-be/internal/adapter/http"
	"github.com/dua-os/dua-os-be/internal/adapter/llm"
	"github.com/dua-os/dua-os-be/internal/adapter/memory"
	"github.com/dua-os/dua-os-be/internal/adapter/ohip"
	"github.com/dua-os/dua-os-be/internal/adapter/outlook"
	pgadapter "github.com/dua-os/dua-os-be/internal/adapter/postgres"
	"github.com/dua-os/dua-os-be/internal/app/actor"
	appapproval "github.com/dua-os/dua-os-be/internal/app/approval"
	appassistant "github.com/dua-os/dua-os-be/internal/app/assistant"
	appauth "github.com/dua-os/dua-os-be/internal/app/auth"
	appbrief "github.com/dua-os/dua-os-be/internal/app/brief"
	appcafecheck "github.com/dua-os/dua-os-be/internal/app/cafecheck"
	appcafedef "github.com/dua-os/dua-os-be/internal/app/cafedef"
	appcafehandover "github.com/dua-os/dua-os-be/internal/app/cafehandover"
	appcafeintake "github.com/dua-os/dua-os-be/internal/app/cafeintake"
	appcafepos "github.com/dua-os/dua-os-be/internal/app/cafepos"
	appcafeprep "github.com/dua-os/dua-os-be/internal/app/cafeprep"
	appcafeqr "github.com/dua-os/dua-os-be/internal/app/cafeqr"
	appcafestock "github.com/dua-os/dua-os-be/internal/app/cafestock"
	appcafewaste "github.com/dua-os/dua-os-be/internal/app/cafewaste"
	appcash "github.com/dua-os/dua-os-be/internal/app/cash"
	appdashboard "github.com/dua-os/dua-os-be/internal/app/dashboard"
	appdecision "github.com/dua-os/dua-os-be/internal/app/decision"
	appdoc "github.com/dua-os/dua-os-be/internal/app/docintake"
	apphotelpms "github.com/dua-os/dua-os-be/internal/app/hotelpms"
	appinbox "github.com/dua-os/dua-os-be/internal/app/inbox"
	appportfolio "github.com/dua-os/dua-os-be/internal/app/portfolio"
	appreport "github.com/dua-os/dua-os-be/internal/app/report"
	appstaff "github.com/dua-os/dua-os-be/internal/app/staff"
	apptask "github.com/dua-os/dua-os-be/internal/app/task"
	"github.com/dua-os/dua-os-be/internal/domain/cafe"
	"github.com/dua-os/dua-os-be/internal/domain/hotel"
	"github.com/dua-os/dua-os-be/internal/domain/supply"
	"github.com/dua-os/dua-os-be/internal/domain/tenancy"
	"github.com/dua-os/dua-os-be/internal/platform/authn"
	"github.com/dua-os/dua-os-be/internal/platform/config"
	"github.com/dua-os/dua-os-be/internal/platform/postgres"
	"github.com/dua-os/dua-os-be/internal/platform/secretbox"
	"github.com/dua-os/dua-os-be/internal/shared/clock"
	"github.com/dua-os/dua-os-be/internal/shared/id"
)

type App struct {
	Server *http.Server
	Pool   *pgxpool.Pool
	Log    *slog.Logger
	Seed   SeedResult
	Memory bool
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	tokens, err := authn.NewJWT(cfg.JWTSecret, cfg.JWTTTL)
	if err != nil {
		return nil, err
	}
	registry := tenancy.NewRegistry(cafe.Catalog{}, hotel.Catalog{}, supply.Catalog{})
	clk := clock.System{}
	passwords := authn.Bcrypt{}
	vault, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	app := &App{Log: log}

	if cfg.DatabaseURL != "" {
		pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		app.Pool = pool
		pg := pgadapter.New(pool)
		log.Info("storage", "driver", "postgres")
		app.Server = compose(cfg, registry, tokens, passwords, clk, vault, pg)
		return app, nil
	}
	if cfg.Production() {
		return nil, fmt.Errorf("DATABASE_URL is required in production")
	}
	mem := memory.New()
	app.Memory = true
	if cfg.DataDir != "" {
		if err := mem.AttachAIPersist(filepath.Join(cfg.DataDir, "assistant-settings.json")); err != nil {
			return nil, fmt.Errorf("assistant persist: %w", err)
		}
		if err := mem.AttachCafePersist(filepath.Join(cfg.DataDir, "cafe-definition.json")); err != nil {
			return nil, fmt.Errorf("cafe persist: %w", err)
		}
		if err := mem.AttachQRPersist(filepath.Join(cfg.DataDir, "cafe-qr.json")); err != nil {
			return nil, fmt.Errorf("qr persist: %w", err)
		}
		if err := mem.AttachIntakePersist(filepath.Join(cfg.DataDir, "cafe-intake.json")); err != nil {
			return nil, fmt.Errorf("intake persist: %w", err)
		}
		if err := mem.AttachHandoverPersist(filepath.Join(cfg.DataDir, "cafe-handover.json")); err != nil {
			return nil, fmt.Errorf("handover persist: %w", err)
		}
		if err := mem.AttachCheckPersist(filepath.Join(cfg.DataDir, "cafe-checklist.json")); err != nil {
			return nil, fmt.Errorf("checklist persist: %w", err)
		}
		if err := mem.AttachApprovalPersist(filepath.Join(cfg.DataDir, "spend-approvals.json")); err != nil {
			return nil, fmt.Errorf("approval persist: %w", err)
		}
		if err := mem.AttachPOSPersist(filepath.Join(cfg.DataDir, "cafe-pos.json")); err != nil {
			return nil, fmt.Errorf("pos persist: %w", err)
		}
		if err := mem.AttachPrepPersist(filepath.Join(cfg.DataDir, "cafe-prep.json")); err != nil {
			return nil, fmt.Errorf("prep persist: %w", err)
		}
		if err := mem.AttachWastePersist(filepath.Join(cfg.DataDir, "cafe-waste.json")); err != nil {
			return nil, fmt.Errorf("waste persist: %w", err)
		}
		if err := mem.AttachStockPersist(filepath.Join(cfg.DataDir, "cafe-stock.json")); err != nil {
			return nil, fmt.Errorf("stock persist: %w", err)
		}
		if err := mem.AttachHotelPersist(filepath.Join(cfg.DataDir, "hotel-pms.json")); err != nil {
			return nil, fmt.Errorf("hotel pms persist: %w", err)
		}
		if err := mem.AttachDocPersist(filepath.Join(cfg.DataDir, "workspace-docs.json")); err != nil {
			return nil, fmt.Errorf("doc persist: %w", err)
		}
		if err := mem.AttachMeetingPersist(filepath.Join(cfg.DataDir, "assistant-meetings.json")); err != nil {
			return nil, fmt.Errorf("meeting persist: %w", err)
		}
	}
	log.Warn("storage", "driver", "memory", "hint", "set DATABASE_URL for postgres")
	seed, err := SeedCafe(ctx, mem, clk.Now())
	if err != nil {
		return nil, err
	}
	hotels, err := SeedPortfolioHotels(ctx, mem, clk.Now())
	if err != nil {
		return nil, err
	}
	if err := restoreHotelNames(ctx, mem, hotels.SheratonID, hotels.AlShohadaID); err != nil {
		return nil, err
	}
	app.Seed = seed
	log.Info("cafe seed", "email", seed.GMEmail, "cafe", seed.CafeID.String(), "sheraton", hotels.SheratonID.String(), "shohada", hotels.AlShohadaID.String())
	app.Server = compose(cfg, registry, tokens, passwords, clk, vault, mem)
	return app, nil
}

// restoreHotelNames re-applies saved hotel profiles over the memory seed, which recreates workspaces on boot.
func restoreHotelNames(ctx context.Context, mem *memory.Store, ids ...id.ID) error {
	for _, wid := range ids {
		p, err := mem.GetHotelProfile(ctx, wid)
		if err != nil {
			continue
		}
		ws, err := mem.GetWorkspace(ctx, wid)
		if err != nil {
			return err
		}
		ws.Name = p.DisplayName
		if p.Timezone != "" {
			ws.Timezone = p.Timezone
		}
		ws.UpdatedAt = p.UpdatedAt
		if err := mem.UpdateWorkspace(ctx, ws); err != nil {
			return err
		}
	}
	return nil
}

type kernelStore interface {
	appauth.UserReader
	appauth.MembershipReader
	appauth.WorkspaceReader
	appstaff.UserWriter
	appstaff.MembershipRepo
	apptask.Repository
	apptask.MembershipReader
	appbrief.TaskStore
	appbrief.MembershipReader
	appbrief.DecisionReader
	appdecision.Repository
	appdecision.TaskStatusReader
	appreport.Repository
	appreport.MembershipReader
	appreport.UserReader
	appcash.Repository
	appapproval.Repository
	appassistant.SettingsRepo
	appassistant.UsageRepo
	appassistant.MeetingRepo
	appcafedef.Repository
	appcafedef.WorkspaceWriter
	appcafeqr.CodeRepo
	appcafeqr.Inbox
	appcafeintake.Repository
	appcafehandover.Repository
	appcafehandover.OpenWork
	appcafecheck.Repository
	appcafepos.Repository
	appcafeprep.Repository
	appcafewaste.Repository
	appcafestock.Repository
	apphotelpms.Repository
	appdoc.Repository
	appinbox.Repository
	appportfolio.Repository
}

type inboxLLM struct{ gateway *llm.Gateway }

func (b inboxLLM) Complete(ctx context.Context, req appinbox.CompletionRequest) (appinbox.CompletionResult, error) {
	res, err := b.gateway.Complete(ctx, appassistant.CompletionRequest{
		Provider: req.Provider, Model: req.Model, APIKey: req.APIKey, System: req.System,
		Messages: req.Messages, MaxOutputTokens: req.MaxOutputTokens, Temperature: req.Temperature,
	})
	if err != nil {
		return appinbox.CompletionResult{}, err
	}
	return appinbox.CompletionResult{Text: res.Text}, nil
}

type taskAssigner struct{ tasks *apptask.Service }

func (a taskAssigner) Assign(ctx context.Context, ac actor.Context, title, description, assigneeUserID string, start, end time.Time) (id.ID, error) {
	view, err := a.tasks.Create(ctx, ac, title, description, assigneeUserID, start, end)
	if err != nil {
		return id.Zero(), err
	}
	return id.Parse(view.ID)
}

func compose(
	cfg config.Config,
	registry *tenancy.Registry,
	tokens *authn.JWT,
	passwords authn.Bcrypt,
	clk clock.Clock,
	vault appassistant.Cipher,
	store kernelStore,
) *http.Server {
	tasks := apptask.New(store, store, clk)
	assigner := taskAssigner{tasks: tasks}
	cafeDef := appcafedef.New(store, store, clk)
	cafeQR := appcafeqr.New(store, store, store, assigner, clk, cfg.PublicAppURL)
	cafeIntake := appcafeintake.New(store, store, store, clk)
	cafeHandover := appcafehandover.New(store, store, store, store, clk)
	cafeCheck := appcafecheck.New(store, store, store, clk)
	cafePOS := appcafepos.New(store, store, clk)
	cafePrep := appcafeprep.New(store, store, clk)
	cafeWaste := appcafewaste.New(store, store, clk)
	cafeStock := appcafestock.New(store, store, clk)
	hotelPMS := apphotelpms.New(store, vault, ohip.NewClient(), clk).WithWorkspaces(store)
	portfolioSvc := appportfolio.New(store, registry, clk)
	briefs := appbrief.New(store, clk,
		appbrief.HandoverSignal{Store: store},
		appbrief.TaskSignal{Tasks: store, Members: store},
		appbrief.VoiceSignal{Source: cafeQR},
		appbrief.BreakdownSignal{Store: store},
		appbrief.IntakeSignal{Store: store},
		appbrief.ChecklistSignal{Store: store},
		appbrief.PrepSignal{Store: store},
		appbrief.WasteSignal{Store: store},
		appbrief.CashSignal{Closes: store},
	)
	reports := appreport.New(store, store, store, assigner, clk)
	dashboard := appdashboard.New(store, store, store, store, store, store, store, store, clk)
	dashboard.SetOccupancyReader(store)
	gateway := llm.NewGateway(cfg.OpenAIBaseURL, cfg.GeminiBaseURL)
	docs := appdoc.New(store, &llm.DocEnricher{Settings: store, Cipher: vault, Gateway: gateway}, clk)
	ctxProvider := assistantctx.New(dashboard, briefs, tasks, reports)
	ctxProvider.SetPortfolio(portfolioSvc)
	assistant := appassistant.New(store, store, vault, gateway, ctxProvider, gateway, clk)
	assistant.SetVoiceCapabilities(gateway, store)
	outlookClient := outlook.New(cfg.MSGraphClientID, cfg.MSGraphClientSecret, cfg.MSGraphTenant, cfg.MSGraphRedirectURL)
	inboxBridge := inboxLLM{gateway}
	outlookFactory := func(clientID, clientSecret, tenant string) appinbox.MailClient {
		return outlook.New(clientID, clientSecret, tenant, cfg.MSGraphRedirectURL)
	}
	inboxSvc := appinbox.New(store, vault, store, store, inboxBridge, outlookClient, outlookFactory, outlook.NewDemo(), clk, cfg.JWTSecret, cfg.PublicAppURL)
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      25 * time.Second,
		Handler: httpadapter.NewRouter(cfg, httpadapter.API{
			Auth:      appauth.New(store, store, store, registry, tokens, passwords, clk),
			Staff:     appstaff.New(store, store, passwords, clk),
			Tasks:     tasks,
			Brief:     briefs,
			Decisions: appdecision.New(store, store, assigner, clk),
			Reports:   reports,
			Cash:      appcash.New(store, clk),
			Approvals: appapproval.New(store, clk),
			Dashboard: dashboard,
			Assistant: assistant,
			Cafe:      cafeDef,
			QR:        cafeQR,
			Intake:    cafeIntake,
			Handover:  cafeHandover,
			Check:     cafeCheck,
			POS:       cafePOS,
			Prep:      cafePrep,
			Waste:     cafeWaste,
			Stock:     cafeStock,
			HotelPMS:  hotelPMS,
			Docs:      docs,
			Portfolio: portfolioSvc,
			Inbox:     inboxSvc,
			Actors:    actor.NewLoader(store, registry),
			JWT:       tokens,
		}),
	}
}

func (a *App) Close() {
	if a.Pool != nil {
		a.Pool.Close()
	}
}
