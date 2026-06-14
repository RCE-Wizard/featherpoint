package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/featherpoint/swinv/internal/server/auth"
	"github.com/featherpoint/swinv/internal/server/store"
)

type claimsKey struct{}

func contextWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

func claimsFromContext(r *http.Request) *auth.Claims {
	if v := r.Context().Value(claimsKey{}); v != nil {
		return v.(*auth.Claims)
	}
	return nil
}

// HandleLogin authenticates a web user and returns a JWT.
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user, err := s.DB.GetUserByUsername(r.Context(), req.Username)
	if err != nil || !store.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := s.Auth.IssueJWT(user.ID, user.Role)
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": user.Role})
}

// jwtMiddleware enforces JWT auth for web API routes.
func (s *Server) jwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := r.Header.Get("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := s.Auth.VerifyJWT(strings.TrimPrefix(hdr, "Bearer "))
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := contextWithClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminOnly rejects non-admin callers.
func adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := claimsFromContext(r)
		if c == nil || c.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Report handlers ---

func (s *Server) HandleHostSoftware(w http.ResponseWriter, r *http.Request) {
	hostID := r.PathValue("hostID")
	source := r.URL.Query().Get("source")
	f := parseFilter(r)
	rows, total, err := s.DB.HostSoftware(r.Context(), hostID, source, f)
	if err != nil {
		log.Printf("HostSoftware: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

func (s *Server) HandleSoftwareHosts(w http.ResponseWriter, r *http.Request) {
	catalogID := r.PathValue("catalogID")
	f := parseFilter(r)
	rows, total, err := s.DB.SoftwareHosts(r.Context(), catalogID, f)
	if err != nil {
		log.Printf("SoftwareHosts: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

func (s *Server) HandleVersionSprawl(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	rows, err := s.DB.VersionSprawl(r.Context(), name)
	if err != nil {
		log.Printf("VersionSprawl: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) HandleUnsigned(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	rows, total, err := s.DB.UnsignedBinaries(r.Context(), f)
	if err != nil {
		log.Printf("UnsignedBinaries: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

func (s *Server) HandleDormant(w http.ResponseWriter, r *http.Request) {
	f := parseFilter(r)
	rows, total, err := s.DB.DormantSoftware(r.Context(), f)
	if err != nil {
		log.Printf("DormantSoftware: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

func (s *Server) HandleCatalogSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	f := parseFilter(r)
	rows, total, err := s.DB.SoftwareCatalogSearch(r.Context(), q, f)
	if err != nil {
		log.Printf("CatalogSearch: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

// --- helpers ---

func parseFilter(r *http.Request) store.ReportFilter {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return store.ReportFilter{
		Limit:  limit,
		Offset: offset,
		Search: r.URL.Query().Get("q"),
	}
}

func writePaged(w http.ResponseWriter, data interface{}, total int) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"data": data, "total": total}); err != nil {
		log.Printf("writePaged: %v", err)
	}
}
