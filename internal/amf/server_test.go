package amf

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go5gc-control-plane-lab/internal/models"
)

func TestRegistrationStoresUEContext(t *testing.T) {
	server := NewServer("amf-001")
	body := mustJSON(t, models.RegistrationRequest{
		SUPI:       "imsi-001010000000001",
		PLMNID:     "00101",
		AccessType: "3GPP_ACCESS",
		RequestedNSSAI: []models.SNSSAI{
			{SST: 1, SD: "010203"},
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/registration", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	var registration models.RegistrationResponse
	if err := json.NewDecoder(response.Body).Decode(&registration); err != nil {
		t.Fatalf("decode registration response: %v", err)
	}
	if registration.GUTI == "" {
		t.Fatal("expected generated guti")
	}
	if registration.RegistrationState != RegistrationStateRegistered {
		t.Fatalf("registration state mismatch: got %s", registration.RegistrationState)
	}
	if registration.ConnectionState != ConnectionStateConnected {
		t.Fatalf("connection state mismatch: got %s", registration.ConnectionState)
	}
	if registration.SecurityContextID == "" {
		t.Fatal("expected security context id")
	}

	lookup := httptest.NewRequest(http.MethodGet, "/ues/imsi-001010000000001", nil)
	lookupResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(lookupResponse, lookup)

	if lookupResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, lookupResponse.Code)
	}

	var ctx UEContext
	if err := json.NewDecoder(lookupResponse.Body).Decode(&ctx); err != nil {
		t.Fatalf("decode ue context: %v", err)
	}
	if ctx.AuthenticationStatus != AuthenticationStatusSuccess {
		t.Fatalf("authentication status mismatch: got %s", ctx.AuthenticationStatus)
	}
	if len(ctx.AllowedNSSAI) != 1 {
		t.Fatalf("expected one allowed nssai, got %d", len(ctx.AllowedNSSAI))
	}
}

func TestRegistrationRejectsInvalidSUPI(t *testing.T) {
	server := NewServer("amf-001")
	body := mustJSON(t, models.RegistrationRequest{
		SUPI:       "bad-supi",
		PLMNID:     "00101",
		AccessType: "3GPP_ACCESS",
	})

	request := httptest.NewRequest(http.MethodPost, "/registration", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestUECollectionListsAndResetsState(t *testing.T) {
	server := NewServer("amf-001")
	registerUE(t, server, "imsi-001010000000001")
	registerUE(t, server, "imsi-001010000000002")

	list := httptest.NewRequest(http.MethodGet, "/ues", nil)
	listResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listResponse.Code)
	}

	var payload UEListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&payload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if payload.Count != 2 {
		t.Fatalf("expected 2 UEs, got %d", payload.Count)
	}

	reset := httptest.NewRequest(http.MethodDelete, "/ues", nil)
	resetResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(resetResponse, reset)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resetResponse.Code)
	}

	listAfterReset := httptest.NewRequest(http.MethodGet, "/ues", nil)
	listAfterResetResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(listAfterResetResponse, listAfterReset)
	var afterReset UEListResponse
	if err := json.NewDecoder(listAfterResetResponse.Body).Decode(&afterReset); err != nil {
		t.Fatalf("decode list after reset: %v", err)
	}
	if afterReset.Count != 0 {
		t.Fatalf("expected empty UE list, got %d", afterReset.Count)
	}
}

func TestConnectionReleaseAndServiceRequestUpdateCMState(t *testing.T) {
	server := NewServer("amf-001")
	supi := "imsi-001010000000001"
	registerUE(t, server, supi)

	release := httptest.NewRequest(http.MethodPost, "/ues/"+supi+"/release", nil)
	releaseResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(releaseResponse, release)
	if releaseResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, releaseResponse.Code)
	}
	assertUEState(t, server, supi, ConnectionStateIdle)

	service := httptest.NewRequest(http.MethodPost, "/ues/"+supi+"/service-request", nil)
	serviceResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(serviceResponse, service)
	if serviceResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, serviceResponse.Code)
	}
	assertUEState(t, server, supi, ConnectionStateConnected)
}

func TestDeregisteredUECannotResumeService(t *testing.T) {
	server := NewServer("amf-001")
	supi := "imsi-001010000000001"
	registerUE(t, server, supi)

	deregister := httptest.NewRequest(http.MethodDelete, "/ues/"+supi, nil)
	deregisterResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(deregisterResponse, deregister)
	if deregisterResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, deregisterResponse.Code)
	}

	service := httptest.NewRequest(http.MethodPost, "/ues/"+supi+"/service-request", nil)
	serviceResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(serviceResponse, service)
	if serviceResponse.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, serviceResponse.Code)
	}
}

func registerUE(t *testing.T, server *Server, supi string) {
	t.Helper()
	body := mustJSON(t, models.RegistrationRequest{
		SUPI:       supi,
		PLMNID:     "00101",
		AccessType: "3GPP_ACCESS",
	})
	request := httptest.NewRequest(http.MethodPost, "/registration", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected registration status %d, got %d", http.StatusCreated, response.Code)
	}
}

func assertUEState(t *testing.T, server *Server, supi string, connectionState string) {
	t.Helper()
	lookup := httptest.NewRequest(http.MethodGet, "/ues/"+supi, nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, lookup)
	if response.Code != http.StatusOK {
		t.Fatalf("expected lookup status %d, got %d", http.StatusOK, response.Code)
	}

	var ctx UEContext
	if err := json.NewDecoder(response.Body).Decode(&ctx); err != nil {
		t.Fatalf("decode ue context: %v", err)
	}
	if ctx.ConnectionState != connectionState {
		t.Fatalf("connection state mismatch: got %s want %s", ctx.ConnectionState, connectionState)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return body
}
