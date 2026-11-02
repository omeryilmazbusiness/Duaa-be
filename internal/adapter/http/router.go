package httpadapter

import (
	"net/http"
	"time"

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
	appcash "github.com/dua-os/dua-os-be/internal/app/cash"
	appdashboard "github.com/dua-os/dua-os-be/internal/app/dashboard"
	appdecision "github.com/dua-os/dua-os-be/internal/app/decision"
	appreport "github.com/dua-os/dua-os-be/internal/app/report"
	appstaff "github.com/dua-os/dua-os-be/internal/app/staff"
	apptask "github.com/dua-os/dua-os-be/internal/app/task"
	"github.com/dua-os/dua-os-be/internal/platform/authn"
	"github.com/dua-os/dua-os-be/internal/platform/config"
	"github.com/dua-os/dua-os-be/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type API struct {
	Auth      *appauth.Service
	Staff     *appstaff.Service
	Tasks     *apptask.Service
	Brief     *appbrief.Service
	Decisions *appdecision.Service
	Reports   *appreport.Service
	Cash      *appcash.Service
	Approvals *appapproval.Service
	Dashboard *appdashboard.Service
	Assistant *appassistant.Service
	Cafe      *appcafedef.Service
	QR        *appcafeqr.Service
	Intake    *appcafeintake.Service
	Handover  *appcafehandover.Service
	Check     *appcafecheck.Service
	POS       *appcafepos.Service
	Prep      *appcafeprep.Service
	Actors    *actor.Loader
	JWT       *authn.JWT
}

func NewRouter(cfg config.Config, api API) http.Handler {
	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(25 * time.Second))
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"version": cfg.Version,
			"env":     cfg.Env,
		})
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/auth/login", api.login)
		r.Post("/auth/login/select", api.loginSelect)
		r.Post("/auth/staff-login", api.staffLogin)

		r.Get("/public/qr/{token}", api.publicQR)
		r.Get("/public/qr/{token}/image", api.publicQRImage)
		r.Post("/public/qr/{token}", api.submitPublicQR)

		r.Group(func(r chi.Router) {
			r.Use(Authenticate(api.JWT))
			r.Get("/me", api.me)
			r.Post("/me/workspaces/{workspaceID}/select", api.switchWorkspace)

			r.Route("/workspaces/{workspaceID}", func(r chi.Router) {
				r.Get("/staff", api.listStaff)
				r.Post("/staff", api.createStaff)
				r.Delete("/staff/{id}", api.deactivateStaff)
				r.Get("/staff/department-managers", api.listDeptManagers)
				r.Post("/staff/department-managers", api.createDeptManager)
				r.Delete("/staff/department-managers/{id}", api.deactivateStaff)

				r.Get("/tasks", api.listTasks)
				r.Post("/tasks", api.createTask)
				r.Patch("/tasks/{id}/complete", api.completeTask)
				r.Get("/routines", api.listRoutines)
				r.Post("/routines", api.createRoutine)

				r.Get("/staff-portal/tasks", api.portalTasks)
				r.Patch("/staff-portal/tasks/{id}/complete", api.completePortalTask)

				r.Get("/reports", api.listReports)
				r.Post("/reports", api.createReport)
				r.Post("/reports/{id}/act", api.actReport)

				r.Get("/executive/brief", api.brief)
				r.Post("/executive/act", api.actDecision)
				r.Get("/executive/dashboard", api.dashboard)

				r.Post("/cash/close", api.closeCash)
				r.Get("/approvals", api.listApprovals)
				r.Post("/approvals", api.createApproval)
				r.Post("/approvals/{id}/act", api.actApproval)
				r.Post("/approvals/{id}/evidence", api.attachApprovalEvidence)

				r.Route("/assistant", func(r chi.Router) {
					r.Get("/status", api.assistantStatus)
					r.Get("/settings", api.assistantSettings)
					r.Get("/models", api.assistantModels)
					r.Put("/settings", api.saveAssistantSettings)
					r.Delete("/settings", api.deleteAssistantSettings)
					r.Post("/chat", api.assistantChat)
				})

				r.Get("/cafe/definition", api.getCafeDefinition)
				r.Put("/cafe/definition", api.putCafeDefinition)
				r.Get("/cafe/qr", api.listCafeQR)
				r.Post("/cafe/qr/tables", api.issueCafeTable)
				r.Patch("/cafe/qr/{siteID}", api.renameCafeTable)
				r.Post("/cafe/qr/{siteID}/rotate", api.rotateCafeQR)
				r.Get("/cafe/voices", api.listCafeVoices)
				r.Post("/cafe/voices/{id}/act", api.actCafeVoice)
				r.Get("/cafe/intake", api.getCafeIntake)
				r.Put("/cafe/intake", api.putCafeIntake)
				r.Get("/cafe/intakes", api.listCafeIntakes)
				r.Get("/cafe/handover", api.getCafeHandover)
				r.Post("/cafe/handover", api.openCafeHandover)
				r.Post("/cafe/handover/{id}/accept", api.acceptCafeHandover)
				r.Get("/cafe/handovers", api.listCafeHandovers)
				r.Get("/cafe/checklist", api.getCafeChecklist)
				r.Put("/cafe/checklist", api.putCafeChecklist)
				r.Post("/cafe/checklist/toggle", api.toggleCafeChecklist)
				r.Get("/cafe/pos", api.getCafePOS)
				r.Post("/cafe/pos/tickets", api.recordCafePOS)
				r.Post("/cafe/pos/close", api.closeCafePOS)
				r.Get("/cafe/prep", api.getCafePrep)
				r.Post("/cafe/prep/tickets", api.recordCafePrep)
				r.Post("/cafe/prep/{id}/ready", api.readyCafePrep)
				r.Post("/cafe/prep/{id}/serve", api.serveCafePrep)
			})
		})
	})
	return r
}

func (a API) actor(w http.ResponseWriter, r *http.Request) (actor.Context, bool) {
	p, err := Principal(r)
	if err != nil {
		httpx.WriteError(w, err)
		return actor.Context{}, false
	}
	ac, err := a.Actors.Load(r.Context(), p, chi.URLParam(r, "workspaceID"))
	if err != nil {
		httpx.WriteError(w, err)
		return actor.Context{}, false
	}
	return ac, true
}
