package smf

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"go5gc-control-plane-lab/internal/amf"
	"go5gc-control-plane-lab/internal/models"
)

type SessionContext struct {
	SUPI       string        `json:"supi"`
	SessionID  int           `json:"session_id"`
	DNN        string        `json:"dnn"`
	SNSSAI     models.SNSSAI `json:"s_nssai"`
	SessionRef string        `json:"session_ref"`
	CreatedAt  time.Time     `json:"created_at"`
}

type SessionListResponse struct {
	Count int              `json:"count"`
	Items []SessionContext `json:"items"`
}

type AMFClient interface {
	GetUEContext(supi string) (amf.UEContext, error)
}

type HTTPAMFClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPAMFClient(baseURL string, client *http.Client) *HTTPAMFClient {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	return &HTTPAMFClient{baseURL: baseURL, client: client}
}

func (c *HTTPAMFClient) GetUEContext(supi string) (amf.UEContext, error) {
	endpoint := fmt.Sprintf("%s/ues/%s", c.baseURL, url.PathEscape(supi))
	resp, err := c.client.Get(endpoint)
	if err != nil {
		return amf.UEContext{}, fmt.Errorf("get ue context: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return amf.UEContext{}, errUENotRegistered
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return amf.UEContext{}, fmt.Errorf("amf returned status %d", resp.StatusCode)
	}

	var ctx amf.UEContext
	if err := json.NewDecoder(resp.Body).Decode(&ctx); err != nil {
		return amf.UEContext{}, fmt.Errorf("decode ue context: %w", err)
	}
	if ctx.RegistrationState != amf.RegistrationStateRegistered {
		return amf.UEContext{}, errUENotRegistered
	}
	return ctx, nil
}

type Server struct {
	smfID     string
	amfClient AMFClient
	mu        sync.RWMutex
	sessions  map[string]map[int]SessionContext
	mux       *http.ServeMux
}

func NewServer(smfID string, amfClient AMFClient) *Server {
	server := &Server{
		smfID:     smfID,
		amfClient: amfClient,
		sessions:  make(map[string]map[int]SessionContext),
		mux:       http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/pdu-sessions", s.handlePDUSessions)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePDUSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListPDUSessions(w)
	case http.MethodPost:
		s.handleCreatePDUSession(w, r)
	case http.MethodDelete:
		s.handleResetPDUSessions(w)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListPDUSessions(w http.ResponseWriter) {
	items := s.listSessions()
	writeJSON(w, http.StatusOK, SessionListResponse{Count: len(items), Items: items})
}

func (s *Server) handleResetPDUSessions(w http.ResponseWriter) {
	s.mu.Lock()
	s.sessions = make(map[string]map[int]SessionContext)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "smf state reset"})
}

func (s *Server) handleCreatePDUSession(w http.ResponseWriter, r *http.Request) {
	var req models.PDUSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := validatePDUSessionRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if _, err := s.amfClient.GetUEContext(req.SUPI); err != nil {
		if errors.Is(err, errUENotRegistered) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered in amf"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to verify ue with amf"})
		return
	}

	ctx, created := s.createSession(req)
	status := http.StatusCreated
	message := "pdu session created"
	if !created {
		status = http.StatusOK
		message = "pdu session already exists"
	}

	writeJSON(w, status, models.PDUSessionResponse{
		Accepted:   true,
		SMFID:      s.smfID,
		SessionRef: ctx.SessionRef,
		Message:    message,
	})
}

func (s *Server) createSession(req models.PDUSessionRequest) (SessionContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[req.SUPI]; !ok {
		s.sessions[req.SUPI] = make(map[int]SessionContext)
	}
	if existing, ok := s.sessions[req.SUPI][req.SessionID]; ok {
		return existing, false
	}

	ctx := SessionContext{
		SUPI:       req.SUPI,
		SessionID:  req.SessionID,
		DNN:        req.DNN,
		SNSSAI:     req.SNSSAI,
		SessionRef: fmt.Sprintf("%s-%d", req.SUPI, req.SessionID),
		CreatedAt:  time.Now().UTC(),
	}
	s.sessions[req.SUPI][req.SessionID] = ctx
	return ctx, true
}

func (s *Server) listSessions() []SessionContext {
	s.mu.RLock()
	items := make([]SessionContext, 0)
	for _, byID := range s.sessions {
		for _, ctx := range byID {
			items = append(items, ctx)
		}
	}
	s.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		if items[i].SUPI == items[j].SUPI {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].SUPI < items[j].SUPI
	})
	return items
}

func validatePDUSessionRequest(req models.PDUSessionRequest) error {
	if req.SUPI == "" {
		return errors.New("supi is required")
	}
	if req.SessionID < 1 {
		return errors.New("session_id must be greater than zero")
	}
	if req.DNN == "" {
		return errors.New("dnn is required")
	}
	if req.SNSSAI.SST < 1 {
		return errors.New("s_nssai.sst must be greater than zero")
	}
	if req.SNSSAI.SD == "" {
		return errors.New("s_nssai.sd is required")
	}
	return nil
}

var errUENotRegistered = errors.New("ue is not registered")

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
