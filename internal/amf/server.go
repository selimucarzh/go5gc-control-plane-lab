package amf

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go5gc-control-plane-lab/internal/models"
)

type UEContext struct {
	UEID         string    `json:"ue_id"`
	SUPI         string    `json:"supi"`
	GUTI         string    `json:"guti,omitempty"`
	PLMNID       string    `json:"plmn_id"`
	AccessType   string    `json:"access_type"`
	RegisteredAt time.Time `json:"registered_at"`
}

type Server struct {
	amfID string
	mu    sync.RWMutex
	ues   map[string]UEContext
	mux   *http.ServeMux
}

func NewServer(amfID string) *Server {
	server := &Server{
		amfID: amfID,
		ues:   make(map[string]UEContext),
		mux:   http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/registration", s.handleRegistration)
	s.mux.HandleFunc("/ues/", s.handleGetUE)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRegistration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req models.RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.SUPI == "" || req.PLMNID == "" || req.AccessType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supi, plmn_id and access_type are required"})
		return
	}

	ctx := UEContext{
		UEID:         fmt.Sprintf("ue-%s", req.SUPI),
		SUPI:         req.SUPI,
		GUTI:         req.GUTI,
		PLMNID:       req.PLMNID,
		AccessType:   req.AccessType,
		RegisteredAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.ues[req.SUPI] = ctx
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, models.RegistrationResponse{
		Accepted: true,
		AMFID:    s.amfID,
		UEID:     ctx.UEID,
		Message:  "registration accepted",
	})
}

func (s *Server) handleGetUE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	supi := strings.TrimPrefix(r.URL.Path, "/ues/")
	if supi == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing supi"})
		return
	}

	s.mu.RLock()
	ctx, ok := s.ues[supi]
	s.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}

	writeJSON(w, http.StatusOK, ctx)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
