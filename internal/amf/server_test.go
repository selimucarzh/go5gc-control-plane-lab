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
	body, err := json.Marshal(models.RegistrationRequest{
		SUPI:       "imsi-001010000000001",
		PLMNID:     "00101",
		AccessType: "3GPP_ACCESS",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/registration", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	lookup := httptest.NewRequest(http.MethodGet, "/ues/imsi-001010000000001", nil)
	lookupResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(lookupResponse, lookup)

	if lookupResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, lookupResponse.Code)
	}
}
