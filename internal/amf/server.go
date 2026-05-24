package amf

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go5gc-control-plane-lab/internal/models"
)

type UEContext struct {
	UEID                 string          `json:"ue_id"`
	SUPI                 string          `json:"supi"`
	GUTI                 string          `json:"guti,omitempty"`
	PLMNID               string          `json:"plmn_id"`
	AccessType           string          `json:"access_type"`
	RegistrationState    string          `json:"registration_state"`
	ConnectionState      string          `json:"connection_state"`
	AuthenticationStatus string          `json:"authentication_status"`
	SecurityContext      SecurityContext `json:"security_context"`
	AllowedNSSAI         []models.SNSSAI `json:"allowed_nssai"`
	RegisteredAt         time.Time       `json:"registered_at"`
	LastSeenAt           time.Time       `json:"last_seen_at"`
	DeregisteredAt       *time.Time      `json:"deregistered_at,omitempty"`
}

type SecurityContext struct {
	ID              string    `json:"id"`
	Algorithm       string    `json:"algorithm"`
	IntegrityKeyRef string    `json:"integrity_key_ref"`
	CipheringKeyRef string    `json:"ciphering_key_ref"`
	EstablishedAt   time.Time `json:"established_at"`
}

type UEListResponse struct {
	Count int         `json:"count"`
	Items []UEContext `json:"items"`
}

type ProcedureResponse struct {
	Accepted bool      `json:"accepted"`
	AMFID    string    `json:"amf_id"`
	SUPI     string    `json:"supi"`
	State    string    `json:"state"`
	Message  string    `json:"message"`
	At       time.Time `json:"at"`
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
	s.mux.HandleFunc("/ues", s.handleUECollection)
	s.mux.HandleFunc("/ues/", s.handleUEResource)
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
	if err := validateRegistrationRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	guti := req.GUTI
	if guti == "" {
		guti = generateGUTI(s.amfID, req.SUPI)
	}
	allowedNSSAI := allowedNSSAI(req.RequestedNSSAI)
	ctx := UEContext{
		UEID:                 fmt.Sprintf("ue-%s", req.SUPI),
		SUPI:                 req.SUPI,
		GUTI:                 guti,
		PLMNID:               req.PLMNID,
		AccessType:           req.AccessType,
		RegistrationState:    RegistrationStateRegistered,
		ConnectionState:      ConnectionStateConnected,
		AuthenticationStatus: AuthenticationStatusSuccess,
		SecurityContext:      newSecurityContext(req.SUPI, now),
		AllowedNSSAI:         allowedNSSAI,
		RegisteredAt:         now,
		LastSeenAt:           now,
	}

	s.mu.Lock()
	s.ues[req.SUPI] = ctx
	s.mu.Unlock()

	writeJSON(w, http.StatusCreated, models.RegistrationResponse{
		Accepted:          true,
		AMFID:             s.amfID,
		UEID:              ctx.UEID,
		GUTI:              ctx.GUTI,
		RegistrationState: ctx.RegistrationState,
		ConnectionState:   ctx.ConnectionState,
		AllowedNSSAI:      ctx.AllowedNSSAI,
		SecurityContextID: ctx.SecurityContext.ID,
		Message:           "registration accepted",
	})
}

func (s *Server) handleUECollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		items := make([]UEContext, 0, len(s.ues))
		for _, ctx := range s.ues {
			items = append(items, ctx)
		}
		s.mu.RUnlock()

		sort.Slice(items, func(i, j int) bool {
			return items[i].SUPI < items[j].SUPI
		})
		writeJSON(w, http.StatusOK, UEListResponse{Count: len(items), Items: items})
	case http.MethodDelete:
		s.mu.Lock()
		s.ues = make(map[string]UEContext)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"status": "amf state reset"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUEResource(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ues/")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing supi"})
		return
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	supi := parts[0]
	if supi == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing supi"})
		return
	}
	if len(parts) == 1 {
		s.handleSingleUE(w, r, supi)
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "service-request":
			s.handleServiceRequest(w, r, supi)
		case "release":
			s.handleConnectionRelease(w, r, supi)
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleSingleUE(w http.ResponseWriter, r *http.Request, supi string) {
	switch r.Method {
	case http.MethodGet:
		ctx, ok := s.getUE(supi)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
			return
		}
		writeJSON(w, http.StatusOK, ctx)
	case http.MethodDelete:
		ctx, ok := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
			ctx.RegistrationState = RegistrationStateDeregistered
			ctx.ConnectionState = ConnectionStateIdle
			ctx.LastSeenAt = now
			ctx.DeregisteredAt = &now
			return ctx
		})
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
			return
		}
		writeJSON(w, http.StatusOK, ProcedureResponse{
			Accepted: true,
			AMFID:    s.amfID,
			SUPI:     ctx.SUPI,
			State:    ctx.RegistrationState,
			Message:  "ue deregistered",
			At:       ctx.LastSeenAt,
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleServiceRequest(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx, ok := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		if ctx.RegistrationState == RegistrationStateRegistered {
			ctx.ConnectionState = ConnectionStateConnected
			ctx.LastSeenAt = now
		}
		return ctx
	})
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}
	if ctx.RegistrationState != RegistrationStateRegistered {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered"})
		return
	}
	writeJSON(w, http.StatusOK, ProcedureResponse{
		Accepted: true,
		AMFID:    s.amfID,
		SUPI:     ctx.SUPI,
		State:    ctx.ConnectionState,
		Message:  "service request accepted",
		At:       ctx.LastSeenAt,
	})
}

func (s *Server) handleConnectionRelease(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx, ok := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		if ctx.RegistrationState == RegistrationStateRegistered {
			ctx.ConnectionState = ConnectionStateIdle
			ctx.LastSeenAt = now
		}
		return ctx
	})
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}
	if ctx.RegistrationState != RegistrationStateRegistered {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered"})
		return
	}
	writeJSON(w, http.StatusOK, ProcedureResponse{
		Accepted: true,
		AMFID:    s.amfID,
		SUPI:     ctx.SUPI,
		State:    ctx.ConnectionState,
		Message:  "connection released to idle",
		At:       ctx.LastSeenAt,
	})
}

func (s *Server) getUE(supi string) (UEContext, bool) {
	s.mu.RLock()
	ctx, ok := s.ues[supi]
	s.mu.RUnlock()
	return ctx, ok
}

func (s *Server) updateUE(supi string, update func(UEContext, time.Time) UEContext) (UEContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.ues[supi]
	if !ok {
		return UEContext{}, false
	}
	ctx = update(ctx, time.Now().UTC())
	s.ues[supi] = ctx
	return ctx, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func validateRegistrationRequest(req models.RegistrationRequest) error {
	if !supiPattern.MatchString(req.SUPI) {
		return errors.New("supi must use imsi- followed by 5 to 15 digits")
	}
	if !plmnPattern.MatchString(req.PLMNID) {
		return errors.New("plmn_id must contain 5 or 6 digits")
	}
	if _, ok := supportedAccessTypes[req.AccessType]; !ok {
		return fmt.Errorf("access_type %s is not supported", req.AccessType)
	}
	return nil
}

func allowedNSSAI(requested []models.SNSSAI) []models.SNSSAI {
	defaultSlice := models.SNSSAI{SST: 1, SD: "010203"}
	if len(requested) == 0 {
		return []models.SNSSAI{defaultSlice}
	}
	allowed := make([]models.SNSSAI, 0, len(requested))
	for _, snssai := range requested {
		if snssai.SST > 0 && snssai.SD != "" {
			allowed = append(allowed, snssai)
		}
	}
	if len(allowed) == 0 {
		return []models.SNSSAI{defaultSlice}
	}
	return allowed
}

func newSecurityContext(supi string, now time.Time) SecurityContext {
	ref := strings.ReplaceAll(supi, "imsi-", "")
	return SecurityContext{
		ID:              "sec-" + ref,
		Algorithm:       "NEA2/NIA2-simulated",
		IntegrityKeyRef: "ik-" + ref,
		CipheringKeyRef: "ck-" + ref,
		EstablishedAt:   now,
	}
}

func generateGUTI(amfID, supi string) string {
	ref := strings.TrimPrefix(supi, "imsi-")
	return fmt.Sprintf("guti-%s-%s", amfID, ref)
}

const (
	RegistrationStateRegistered   = "REGISTERED"
	RegistrationStateDeregistered = "DEREGISTERED"
	ConnectionStateConnected      = "CM_CONNECTED"
	ConnectionStateIdle           = "CM_IDLE"
	AuthenticationStatusSuccess   = "SUCCESS"
)

var (
	supiPattern = regexp.MustCompile(`^imsi-[0-9]{5,15}$`)
	plmnPattern = regexp.MustCompile(`^[0-9]{5,6}$`)

	supportedAccessTypes = map[string]struct{}{
		"3GPP_ACCESS":     {},
		"NON_3GPP_ACCESS": {},
	}
)
