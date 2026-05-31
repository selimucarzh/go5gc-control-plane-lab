package amf

import (
	"bytes"
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

type AuthenticationVector struct {
	RAND      string    `json:"rand"`
	AUTN      string    `json:"autn"`
	XRESStar  string    `json:"xres_star"`
	Generated time.Time `json:"generated"`
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

type AuthenticationResponse struct {
	Accepted bool                 `json:"accepted"`
	AMFID    string               `json:"amf_id"`
	SUPI     string               `json:"supi"`
	Status   string               `json:"status"`
	Vector   AuthenticationVector `json:"vector,omitempty"`
	Message  string               `json:"message"`
	At       time.Time            `json:"at"`
}

type AuthenticationConfirmRequest struct {
	RESStar string `json:"res_star"`
}

type NASMessageRequest struct {
	ProtocolDiscriminator string                      `json:"protocol_discriminator,omitempty"`
	SecurityHeaderType    string                      `json:"security_header_type,omitempty"`
	MessageType           string                      `json:"message_type"`
	SequenceNumber        int                         `json:"sequence_number,omitempty"`
	SUPI                  string                      `json:"supi,omitempty"`
	Payload               *NASMessagePayload          `json:"payload,omitempty"`
	Registration          *models.RegistrationRequest `json:"registration,omitempty"`
}

type NASMessagePayload struct {
	SUPI                 string          `json:"supi,omitempty"`
	GUTI                 string          `json:"guti,omitempty"`
	PLMNID               string          `json:"plmn_id,omitempty"`
	AccessType           string          `json:"access_type,omitempty"`
	RegistrationType     string          `json:"registration_type,omitempty"`
	NGKSI                int             `json:"ngksi,omitempty"`
	RequestedNSSAI       []models.SNSSAI `json:"requested_nssai,omitempty"`
	UEMMCapability       []string        `json:"ue_mm_capability,omitempty"`
	UESecurityCapability []string        `json:"ue_security_capability,omitempty"`
}

type ProcedureEvent struct {
	ID          int       `json:"id"`
	SUPI        string    `json:"supi,omitempty"`
	Procedure   string    `json:"procedure"`
	Step        string    `json:"step"`
	Direction   string    `json:"direction"`
	MessageType string    `json:"message_type"`
	Detail      string    `json:"detail"`
	At          time.Time `json:"at"`
}

type EventListResponse struct {
	Count int              `json:"count"`
	Items []ProcedureEvent `json:"items"`
}

type Server struct {
	amfID       string
	mu          sync.RWMutex
	ues         map[string]UEContext
	authVectors map[string]AuthenticationVector
	events      []ProcedureEvent
	nextEventID int
	mux         *http.ServeMux
}

func NewServer(amfID string) *Server {
	server := &Server{
		amfID:       amfID,
		ues:         make(map[string]UEContext),
		authVectors: make(map[string]AuthenticationVector),
		mux:         http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/nas", s.handleNASMessage)
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

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	items := append([]ProcedureEvent(nil), s.events...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, EventListResponse{Count: len(items), Items: items})
}

func (s *Server) handleNASMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req NASMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	switch req.MessageType {
	case NASMessageRegistrationRequest:
		registration, err := req.registrationRequest()
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := validateNASEnvelope(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.handleRegistrationProcedure(w, registration)
	case NASMessageServiceRequest:
		supi := req.supi()
		if supi == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supi is required"})
			return
		}
		if err := validateNASEnvelope(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.acceptServiceRequest(w, supi)
	case NASMessageDeregistrationRequest:
		supi := req.supi()
		if supi == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supi is required"})
			return
		}
		if err := validateNASEnvelope(req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		s.acceptDeregistration(w, supi)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported nas message_type"})
	}
}

func (req NASMessageRequest) registrationRequest() (models.RegistrationRequest, error) {
	if req.Payload != nil {
		if req.Payload.SUPI == "" || req.Payload.PLMNID == "" || req.Payload.AccessType == "" {
			return models.RegistrationRequest{}, errors.New("payload.supi, payload.plmn_id and payload.access_type are required")
		}
		return models.RegistrationRequest{
			SUPI:           req.Payload.SUPI,
			GUTI:           req.Payload.GUTI,
			PLMNID:         req.Payload.PLMNID,
			AccessType:     req.Payload.AccessType,
			RequestedNSSAI: req.Payload.RequestedNSSAI,
		}, nil
	}
	if req.Registration == nil {
		return models.RegistrationRequest{}, errors.New("registration payload is required")
	}
	return *req.Registration, nil
}

func (req NASMessageRequest) supi() string {
	if req.Payload != nil && req.Payload.SUPI != "" {
		return req.Payload.SUPI
	}
	return req.SUPI
}

func validateNASEnvelope(req NASMessageRequest) error {
	if req.ProtocolDiscriminator != "" && req.ProtocolDiscriminator != NASProtocolDiscriminator5GMM {
		return fmt.Errorf("protocol_discriminator %s is not supported", req.ProtocolDiscriminator)
	}
	if req.SecurityHeaderType != "" {
		if _, ok := supportedSecurityHeaderTypes[req.SecurityHeaderType]; !ok {
			return fmt.Errorf("security_header_type %s is not supported", req.SecurityHeaderType)
		}
	}
	if req.SequenceNumber < 0 {
		return errors.New("sequence_number must be zero or greater")
	}
	return nil
}

func (req *NASMessageRequest) UnmarshalJSON(data []byte) error {
	type alias NASMessageRequest
	var decoded struct {
		alias
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*req = NASMessageRequest(decoded.alias)
	if len(decoded.Payload) == 0 || bytes.Equal(decoded.Payload, []byte("null")) {
		return nil
	}

	switch req.MessageType {
	case NASMessageRegistrationRequest:
		var payload NASMessagePayload
		if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
			return err
		}
		req.Payload = &payload
	case NASMessageServiceRequest, NASMessageDeregistrationRequest:
		var payload NASMessagePayload
		if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
			return err
		}
		req.Payload = &payload
	default:
		var payload NASMessagePayload
		if err := json.Unmarshal(decoded.Payload, &payload); err == nil {
			req.Payload = &payload
		}
	}
	return nil
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

	s.handleRegistrationProcedure(w, req)
}

func (s *Server) handleRegistrationProcedure(w http.ResponseWriter, req models.RegistrationRequest) {
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
	securityContext := newSecurityContext(req.SUPI, now)
	ctx := UEContext{
		UEID:                 fmt.Sprintf("ue-%s", req.SUPI),
		SUPI:                 req.SUPI,
		GUTI:                 guti,
		PLMNID:               req.PLMNID,
		AccessType:           req.AccessType,
		RegistrationState:    RegistrationStateRegistered,
		ConnectionState:      ConnectionStateConnected,
		AuthenticationStatus: AuthenticationStatusSuccess,
		SecurityContext:      securityContext,
		AllowedNSSAI:         allowedNSSAI,
		RegisteredAt:         now,
		LastSeenAt:           now,
	}

	s.mu.Lock()
	s.ues[req.SUPI] = ctx
	s.appendEventLocked(req.SUPI, "Registration", "accept", "UE->AMF", "RegistrationRequest", "UE context stored and security context established", now)
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
		s.authVectors = make(map[string]AuthenticationVector)
		s.events = nil
		s.nextEventID = 0
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
		case "authentication":
			s.handleAuthenticationStart(w, r, supi)
		case "service-request":
			s.handleServiceRequest(w, r, supi)
		case "release":
			s.handleConnectionRelease(w, r, supi)
		case "events":
			s.handleUEEvents(w, r, supi)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if len(parts) == 3 && parts[1] == "authentication" && parts[2] == "confirm" {
		s.handleAuthenticationConfirm(w, r, supi)
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
		s.acceptDeregistration(w, supi)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAuthenticationStart(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	current, ok := s.getUE(supi)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}
	if current.RegistrationState != RegistrationStateRegistered {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered"})
		return
	}
	vector := newAuthenticationVector(supi, time.Now().UTC())
	ctx, _ := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		ctx.AuthenticationStatus = AuthenticationStatusChallengeSent
		ctx.LastSeenAt = now
		return ctx
	}, "Authentication", "challenge", "AMF->UE", "AuthenticationRequest", "Authentication vector generated")

	s.mu.Lock()
	s.authVectors[supi] = vector
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, AuthenticationResponse{
		Accepted: true,
		AMFID:    s.amfID,
		SUPI:     supi,
		Status:   ctx.AuthenticationStatus,
		Vector:   vector,
		Message:  "authentication challenge generated",
		At:       ctx.LastSeenAt,
	})
}

func (s *Server) handleAuthenticationConfirm(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req AuthenticationConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.mu.RLock()
	vector, hasVector := s.authVectors[supi]
	s.mu.RUnlock()
	if !hasVector {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "authentication challenge is missing"})
		return
	}
	if req.RESStar != vector.XRESStar {
		_, _ = s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
			ctx.AuthenticationStatus = AuthenticationStatusFailed
			ctx.LastSeenAt = now
			return ctx
		}, "Authentication", "reject", "UE->AMF", "AuthenticationResponse", "Authentication response did not match expected result")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
		return
	}

	ctx, ok := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		ctx.AuthenticationStatus = AuthenticationStatusSuccess
		ctx.SecurityContext = newSecurityContext(supi, now)
		ctx.LastSeenAt = now
		return ctx
	}, "Authentication", "confirm", "UE->AMF", "AuthenticationResponse", "Authentication response accepted and security context refreshed")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}

	writeJSON(w, http.StatusOK, AuthenticationResponse{
		Accepted: true,
		AMFID:    s.amfID,
		SUPI:     supi,
		Status:   ctx.AuthenticationStatus,
		Message:  "authentication accepted",
		At:       ctx.LastSeenAt,
	})
}

func (s *Server) handleServiceRequest(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.acceptServiceRequest(w, supi)
}

func (s *Server) acceptServiceRequest(w http.ResponseWriter, supi string) {
	current, ok := s.getUE(supi)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}
	if current.RegistrationState != RegistrationStateRegistered {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered"})
		return
	}
	ctx, _ := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		ctx.ConnectionState = ConnectionStateConnected
		ctx.LastSeenAt = now
		return ctx
	}, "ServiceRequest", "accept", "UE->AMF", "ServiceRequest", "UE connection state moved to CM_CONNECTED")
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
	current, ok := s.getUE(supi)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ue not found"})
		return
	}
	if current.RegistrationState != RegistrationStateRegistered {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ue is not registered"})
		return
	}
	ctx, _ := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		ctx.ConnectionState = ConnectionStateIdle
		ctx.LastSeenAt = now
		return ctx
	}, "ConnectionRelease", "accept", "AMF->UE", "UEContextReleaseCommand", "UE connection state moved to CM_IDLE")
	writeJSON(w, http.StatusOK, ProcedureResponse{
		Accepted: true,
		AMFID:    s.amfID,
		SUPI:     ctx.SUPI,
		State:    ctx.ConnectionState,
		Message:  "connection released to idle",
		At:       ctx.LastSeenAt,
	})
}

func (s *Server) acceptDeregistration(w http.ResponseWriter, supi string) {
	ctx, ok := s.updateUE(supi, func(ctx UEContext, now time.Time) UEContext {
		ctx.RegistrationState = RegistrationStateDeregistered
		ctx.ConnectionState = ConnectionStateIdle
		ctx.LastSeenAt = now
		ctx.DeregisteredAt = &now
		return ctx
	}, "Deregistration", "accept", "UE->AMF", NASMessageDeregistrationRequest, "UE registration state moved to DEREGISTERED")
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
}

func (s *Server) handleUEEvents(w http.ResponseWriter, r *http.Request, supi string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	items := make([]ProcedureEvent, 0)
	for _, event := range s.events {
		if event.SUPI == supi {
			items = append(items, event)
		}
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, EventListResponse{Count: len(items), Items: items})
}

func (s *Server) getUE(supi string) (UEContext, bool) {
	s.mu.RLock()
	ctx, ok := s.ues[supi]
	s.mu.RUnlock()
	return ctx, ok
}

func (s *Server) updateUE(
	supi string,
	update func(UEContext, time.Time) UEContext,
	procedure string,
	step string,
	direction string,
	messageType string,
	detail string,
) (UEContext, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.ues[supi]
	if !ok {
		return UEContext{}, false
	}
	now := time.Now().UTC()
	ctx = update(ctx, now)
	s.ues[supi] = ctx
	if procedure != "" {
		s.appendEventLocked(supi, procedure, step, direction, messageType, detail, now)
	}
	return ctx, true
}

func (s *Server) appendEventLocked(supi, procedure, step, direction, messageType, detail string, now time.Time) {
	s.nextEventID++
	s.events = append(s.events, ProcedureEvent{
		ID:          s.nextEventID,
		SUPI:        supi,
		Procedure:   procedure,
		Step:        step,
		Direction:   direction,
		MessageType: messageType,
		Detail:      detail,
		At:          now,
	})
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

func newAuthenticationVector(supi string, now time.Time) AuthenticationVector {
	ref := strings.TrimPrefix(supi, "imsi-")
	return AuthenticationVector{
		RAND:      "rand-" + ref,
		AUTN:      "autn-" + ref,
		XRESStar:  "xres-" + ref,
		Generated: now,
	}
}

func generateGUTI(amfID, supi string) string {
	ref := strings.TrimPrefix(supi, "imsi-")
	return fmt.Sprintf("guti-%s-%s", amfID, ref)
}

const (
	RegistrationStateRegistered       = "REGISTERED"
	RegistrationStateDeregistered     = "DEREGISTERED"
	ConnectionStateConnected          = "CM_CONNECTED"
	ConnectionStateIdle               = "CM_IDLE"
	AuthenticationStatusSuccess       = "SUCCESS"
	AuthenticationStatusChallengeSent = "CHALLENGE_SENT"
	AuthenticationStatusFailed        = "FAILED"

	NASMessageRegistrationRequest   = "RegistrationRequest"
	NASMessageServiceRequest        = "ServiceRequest"
	NASMessageDeregistrationRequest = "DeregistrationRequest"

	NASProtocolDiscriminator5GMM       = "5GMM"
	NASSecurityHeaderPlain             = "plain_5gs_nas_message"
	NASSecurityHeaderIntegrity         = "integrity_protected"
	NASSecurityHeaderIntegrityCiphered = "integrity_protected_and_ciphered"
)

var (
	supiPattern = regexp.MustCompile(`^imsi-[0-9]{5,15}$`)
	plmnPattern = regexp.MustCompile(`^[0-9]{5,6}$`)

	supportedAccessTypes = map[string]struct{}{
		"3GPP_ACCESS":     {},
		"NON_3GPP_ACCESS": {},
	}

	supportedSecurityHeaderTypes = map[string]struct{}{
		NASSecurityHeaderPlain:             {},
		NASSecurityHeaderIntegrity:         {},
		NASSecurityHeaderIntegrityCiphered: {},
	}
)
