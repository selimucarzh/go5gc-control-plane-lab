package cpstub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"go5gc-control-plane-lab/internal/controlplane"
)

type Server struct {
	logger *log.Logger
	store  *Store
	mux    *http.ServeMux
}

type Store struct {
	mu       sync.Mutex
	ues      map[string]UERecord
	sessions map[string]map[int]SessionRecord
}

type UERecord struct {
	IMSI         string    `json:"imsi"`
	SUPI         string    `json:"supi"`
	GNBID        string    `json:"gnb_id"`
	RegisteredAt time.Time `json:"registered_at"`
}

type SessionRecord struct {
	SessionID  int                  `json:"session_id"`
	DNN        string               `json:"dnn"`
	SNSSAI     controlplane.SNSSAI  `json:"s_nssai"`
	SessionRef string               `json:"session_ref"`
	CreatedAt  time.Time            `json:"created_at"`
}

func NewServer(logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	server := &Server{
		logger: logger,
		store: &Store{
			ues:      make(map[string]UERecord),
			sessions: make(map[string]map[int]SessionRecord),
		},
		mux: http.NewServeMux(),
	}

	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.methodHandler(http.MethodGet, s.handleHealthz))
	s.mux.HandleFunc("/registration", s.methodHandler(http.MethodPost, s.handleRegistration))
	s.mux.HandleFunc("/pdu-sessions", s.methodHandler(http.MethodPost, s.handlePDUSession))
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) methodHandler(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			s.writeJSON(w, http.StatusMethodNotAllowed, controlplane.Response{
				Status:  "error",
				Message: fmt.Sprintf("method %s not allowed", r.Method),
			})
			return
		}

		next(w, r)
	}
}

func (s *Server) handleRegistration(w http.ResponseWriter, r *http.Request) {
	var req controlplane.RegistrationRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid registration request", err)
		return
	}

	if err := validateRegistrationRequest(req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid registration request", err)
		return
	}

	record, created := s.store.upsertUE(req)
	statusCode := http.StatusCreated
	response := controlplane.Response{
		Status:  "registered",
		Message: fmt.Sprintf("UE %s registered on %s", record.SUPI, record.GNBID),
	}
	if !created {
		statusCode = http.StatusOK
		response.Status = "already-registered"
		response.Message = fmt.Sprintf("UE %s already registered on %s", record.SUPI, record.GNBID)
	}

	s.logger.Printf("event=registration status=%s supi=%s imsi=%s gnb_id=%s", response.Status, record.SUPI, record.IMSI, record.GNBID)
	s.writeJSON(w, statusCode, response)
}

func (s *Server) handlePDUSession(w http.ResponseWriter, r *http.Request) {
	var req controlplane.PDUSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid pdu session request", err)
		return
	}

	if err := validatePDUSessionRequest(req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid pdu session request", err)
		return
	}

	record, err := s.store.createSession(req)
	if err != nil {
		switch {
		case errors.Is(err, errUENotRegistered):
			s.writeJSON(w, http.StatusConflict, controlplane.Response{
				Status:  "rejected",
				Message: err.Error(),
			})
		case errors.Is(err, errSessionAlreadyExists):
			s.writeJSON(w, http.StatusOK, controlplane.Response{
				Status:     "pdu-session-already-exists",
				Message:    err.Error(),
				SessionRef: sessionRef(req.SUPI, req.SessionID),
			})
		default:
			s.writeError(w, http.StatusInternalServerError, "failed to create session", err)
		}
		return
	}

	response := controlplane.Response{
		Status:     "pdu-session-created",
		Message:    fmt.Sprintf("PDU session %d created for %s on DNN %s", req.SessionID, req.SUPI, req.DNN),
		SessionRef: record.SessionRef,
	}

	s.logger.Printf("event=pdu-session status=%s supi=%s session_id=%d dnn=%s session_ref=%s", response.Status, req.SUPI, req.SessionID, req.DNN, record.SessionRef)
	s.writeJSON(w, http.StatusCreated, response)
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string, err error) {
	s.logger.Printf("event=error status_code=%d message=%q error=%q", statusCode, message, err.Error())
	s.writeJSON(w, statusCode, controlplane.Response{
		Status:  "error",
		Message: fmt.Sprintf("%s: %s", message, err.Error()),
	})
}

func (s *Store) upsertUE(req controlplane.RegistrationRequest) (UERecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record, exists := s.ues[req.SUPI]; exists {
		return record, false
	}

	record := UERecord{
		IMSI:         req.IMSI,
		SUPI:         req.SUPI,
		GNBID:        req.GNBID,
		RegisteredAt: time.Now().UTC(),
	}
	s.ues[req.SUPI] = record
	return record, true
}

var (
	errUENotRegistered      = errors.New("ue is not registered")
	errSessionAlreadyExists = errors.New("pdu session already exists for ue and session id")
)

func (s *Store) createSession(req controlplane.PDUSessionRequest) (SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.ues[req.SUPI]; !exists {
		return SessionRecord{}, errUENotRegistered
	}

	if _, exists := s.sessions[req.SUPI]; !exists {
		s.sessions[req.SUPI] = make(map[int]SessionRecord)
	}

	if record, exists := s.sessions[req.SUPI][req.SessionID]; exists {
		return record, errSessionAlreadyExists
	}

	record := SessionRecord{
		SessionID:  req.SessionID,
		DNN:        req.DNN,
		SNSSAI:     req.SNSSAI,
		SessionRef: sessionRef(req.SUPI, req.SessionID),
		CreatedAt:  time.Now().UTC(),
	}
	s.sessions[req.SUPI][req.SessionID] = record
	return record, nil
}

func validateRegistrationRequest(req controlplane.RegistrationRequest) error {
	if req.IMSI == "" {
		return errors.New("imsi is required")
	}
	if req.SUPI == "" {
		return errors.New("supi is required")
	}
	if req.GNBID == "" {
		return errors.New("gnb_id is required")
	}
	return nil
}

func validatePDUSessionRequest(req controlplane.PDUSessionRequest) error {
	if req.IMSI == "" {
		return errors.New("imsi is required")
	}
	if req.SUPI == "" {
		return errors.New("supi is required")
	}
	if req.SessionID < 1 {
		return errors.New("session_id must be greater than zero")
	}
	if req.DNN == "" {
		return errors.New("dnn is required")
	}
	if req.SNSSAI.SD == "" {
		return errors.New("s_nssai.sd is required")
	}
	return nil
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return errors.New("request body must contain a single JSON object")
	}

	return errors.New("request body must contain a single JSON object")
}

func sessionRef(supi string, sessionID int) string {
	return fmt.Sprintf("%s-%d", supi, sessionID)
}

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"status":"error","message":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}
