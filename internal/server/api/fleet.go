package api

import (
	"log"
	"net/http"
	"strconv"
)

func (s *Server) HandleCreateCommand(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	actor := actorFromContext(r)

	var req struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var cmdID string
	var err error

	switch req.Type {
	case "decommission":
		err = s.DB.DecommissionAgent(r.Context(), agentID)
	case "config_update":
		err = s.DB.PushConfig(r.Context(), agentID, req.Payload)
	default:
		cmdID, err = s.DB.CreateCommand(r.Context(), agentID, req.Type, req.Payload)
	}

	if err != nil {
		log.Printf("command %s: %v", req.Type, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_ = s.DB.WriteAuditLog(r.Context(), actor, "command:"+req.Type, agentID, req.Payload)
	writeJSON(w, http.StatusCreated, map[string]string{"id": cmdID})
}

func (s *Server) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFromContext(r)

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

func (s *Server) HandleAuditLog(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, total, err := s.DB.AuditLog(r.Context(), limit, offset)
	if err != nil {
		log.Printf("AuditLog: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writePaged(w, rows, total)
}

func (s *Server) HandleGetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	agent, err := s.DB.GetAgentByID(r.Context(), agentID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func actorFromContext(r *http.Request) string {
	if c := claimsFromContext(r); c != nil {
		return c.Sub
	}
	return "unknown"
}
