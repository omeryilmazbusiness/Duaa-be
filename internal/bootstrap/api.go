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
	appcafeqr "github.com/dua-os/dua-os-be/internal/app/cafeqr"
	appcash "github.com/dua-os/dua-os-be/internal/app/cash"
	appdashboard "github.com/dua-os/dua-os-be/internal/app/dashboard"
	appdecision "github.com/dua-os/dua-os-be/internal/app/decision"
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
	}
	log.Warn("storage", "driver", "memory", "hint", "set DATABASE_URL for postgres")
	seed, err := SeedCafe(ctx, mem, clk.Now())
	if err != nil {
		return nil, err
	}
	app.Seed = seed
	log.Info("cafe seed", "email", seed.GMEmail, "cafe", seed.CafeID.String())
	app.Server = compose(cfg, registry, tokens, passwords, clk, vault, mem)
	return app, nil
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
	appcafedef.Repository
	appcafedef.WorkspaceWriter
	appcafeqr.CodeRepo
	appcafeqr.Inbox
	appcafeintake.Repository
	appcafehandover.Repository
	appcafehandover.OpenWork
	appcafecheck.Repository
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
	briefs := appbrief.New(store, clk,
		appbrief.HandoverSignal{Store: store},
		appbrief.TaskSignal{Tasks: store, Members: store},
		appbrief.VoiceSignal{Source: cafeQR},
		appbrief.BreakdownSignal{Store: store},
		appbrief.IntakeSignal{Store: store},
		appbrief.ChecklistSignal{Store: store},
		appbrief.CashSignal{Closes: store},
	)
	reports := appreport.New(store, store, store, assigner, clk)
	dashboard := appdashboard.New(store, store, store, store, store, store, clk)
	gateway := llm.NewGateway(cfg.OpenAIBaseURL, cfg.GeminiBaseURL)
	assistant := appassistant.New(store, store, vault, gateway,
		assistantctx.New(dashboard, briefs, tasks, reports), gateway, clk)
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
