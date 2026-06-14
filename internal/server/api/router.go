package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/featherpoint/swinv/internal/server/auth"
	"github.com/featherpoint/swinv/internal/server/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(db *store.DB, authSvc *auth.Service) *chi.Mux {
	s := &Server{DB: db, Auth: authSvc}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", handleHealth)

	// Agent endpoints
	r.Route("/v1", func(r chi.Router) {
		r.Post("/enroll", s.HandleEnroll)
		r.Group(func(r chi.Router) {
			r.Use(s.mTLSMiddleware)
			r.Post("/ingest", s.HandleIngest)
			r.Post("/checkin", s.HandleCheckin)
			r.Post("/heartbeat", s.HandleHeartbeat)
		})
	})

	// Web API
	r.Route("/api", func(r chi.Router) {
		r.Post("/login", s.HandleLogin)

		// All other /api routes require JWT
		r.Group(func(r chi.Router) {
			r.Use(s.jwtMiddleware)

			// Fleet
			r.Get("/agents", s.HandleListHosts)
			r.Get("/agents/{agentID}", s.HandleGetAgent)
			r.Get("/audit", s.HandleAuditLog)

			// Reports (viewer + admin)
			r.Get("/hosts/{hostID}/software", s.HandleHostSoftware)
			r.Get("/catalog/{catalogID}/hosts", s.HandleSoftwareHosts)
			r.Get("/reports/version-sprawl", s.HandleVersionSprawl)
			r.Get("/reports/unsigned", s.HandleUnsigned)
			r.Get("/reports/dormant", s.HandleDormant)
			r.Get("/catalog", s.HandleCatalogSearch)

			// Fleet management (admin only)
			r.Group(func(r chi.Router) {
				r.Use(adminOnly)
				r.Post("/agents/{agentID}/commands", s.HandleCreateCommand)
				r.Post("/users", s.HandleCreateUser)
			})
		})
	})

	return r
}

// mTLSMiddleware enforces mTLS for all agent endpoints.
// It verifies the client cert against the CA and maps the fingerprint to an agent row.
// The X-Agent-ID header fallback has been removed — all agent traffic must be mTLS.
func (s *Server) mTLSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "mTLS required", http.StatusUnauthorized)
			return
		}
		cert := r.TLS.PeerCertificates[0]
		fp := sha256.Sum256(cert.Raw)
		fingerprint := hex.EncodeToString(fp[:])
		agent, err := s.DB.GetAgentByCert(r.Context(), fingerprint)
		if err != nil || agent == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := contextWithAgentID(r.Context(), agent.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
