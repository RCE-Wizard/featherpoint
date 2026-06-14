package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/featherpoint/swinv/internal/proto"
	"github.com/featherpoint/swinv/internal/server/auth"
	"github.com/featherpoint/swinv/internal/server/store"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	DB   *store.DB
	Auth *auth.Service
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *Server) HandleEnroll(w http.ResponseWriter, r *http.Request) {
	var req proto.EnrollRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !s.Auth.ValidEnrollmentToken(req.EnrollmentToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	clientCertPEM, fingerprint, err := s.Auth.SignCSR(req.CSRPEM)
	if err != nil {
		log.Printf("sign CSR: %v", err)
		http.Error(w, "csr signing failed", http.StatusInternalServerError)
		return
	}

	result, err := s.DB.Enroll(r.Context(), &req, fingerprint)
	if err != nil {
		log.Printf("enroll: %v", err)
		http.Error(w, "enrollment failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, proto.EnrollResponse{
		AgentID:       result.AgentID,
		ClientCertPEM: clientCertPEM,
		CAPEM:         s.Auth.CAPEM(),
	})
}

func (s *Server) HandleIngest(w http.ResponseWriter, r *http.Request) {
	agentID := agentIDFromContext(r)

	var req proto.IngestRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	seen, err := s.DB.IsBatchSeen(r.Context(), agentID, req.BatchID)
	if err != nil {
		log.Printf("batch check: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if seen {
		writeJSON(w, http.StatusOK, proto.IngestResponse{Accepted: true})
		return
	}

	if err := s.DB.IngestBatch(r.Context(), agentID, &req); err != nil {
		log.Printf("ingest: %v", err)
		http.Error(w, "ingest failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, proto.IngestResponse{Accepted: true})
}

func (s *Server) HandleCheckin(w http.ResponseWriter, r *http.Request) {
	agentID := agentIDFromContext(r)

	var req proto.CheckinRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	_ = s.DB.TouchCheckin(r.Context(), agentID)
	_ = s.DB.AckCommands(r.Context(), agentID, req.AckedCommandIDs)

	agent, err := s.DB.GetAgentByID(r.Context(), agentID)
	if err != nil {
		log.Printf("get agent: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	cmds, _ := s.DB.PendingCommands(r.Context(), agentID)

	resp := proto.CheckinResponse{
		Commands:      cmds,
		ConfigVersion: agent.ConfigVersion,
	}
	if agent.ConfigVersion > req.ConfigVersion {
		resp.Config = agent.Config
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	agentID := agentIDFromContext(r)

	var req proto.HeartbeatRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	_ = s.DB.TouchHeartbeat(r.Context(), agentID, req.Metrics)
	writeJSON(w, http.StatusOK, proto.HeartbeatResponse{OK: true})
}

func (s *Server) HandleListHosts(w http.ResponseWriter, r *http.Request) {
	agents, err := s.DB.ListAgents(r.Context())
	if err != nil {
		log.Printf("list agents: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if agents == nil {
		agents = []map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, agents)
}
