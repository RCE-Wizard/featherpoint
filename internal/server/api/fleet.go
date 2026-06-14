package api

import (
	"log"
	"net/http"
)

// HandleCreateCommand creates a management command for an agent (admin only).
func (s *Server) HandleCreateCommand(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	actor := ""
	if c := claimsFromContext(r); c != nil {
		actor = c.Sub
	}

	var req struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	cmdID, err := s.DB.CreateCommand(r.Context(), agentID, req.Type, req.Payload)
	if err != nil {
		log.Printf("CreateCommand: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = s.DB.WriteAuditLog(r.Context(), actor, "create_command", agentID, map[string]interface{}{
		"command_id": cmdID, "type": req.Type,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": cmdID})
}

// HandleCreateUser creates a new web user (admin only).
func (s *Server) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	actor := ""
	if c := claimsFromContext(r); c != nil {
		actor = c.Sub
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Role != "viewer" && req.Role != "admin" {
		http.Error(w, "role must be viewer or admin", http.StatusBadRequest)
		return
	}

	if err := s.DB.CreateUser(r.Context(), req.Username, req.Password, req.Role); err != nil {
		log.Printf("CreateUser: %v", err)
		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}

	_ = s.DB.WriteAuditLog(r.Context(), actor, "create_user", req.Username, nil)
	writeJSON(w, http.StatusCreated, map[string]string{"username": req.Username, "role": req.Role})
}
