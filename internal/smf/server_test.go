package smf

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go5gc-control-plane-lab/internal/amf"
	"go5gc-control-plane-lab/internal/models"
)

func TestCreatePDUSessionForRegisteredUE(t *testing.T) {
	amfServer := amf.NewServer("amf-001")
	amfHTTP := httptest.NewServer(amfServer.Handler())
	defer amfHTTP.Close()

	registerUE(t, amfHTTP.URL)

	smfServer := NewServer("smf-001", NewHTTPAMFClient(amfHTTP.URL, nil))
	body := mustJSON(t, models.PDUSessionRequest{
		SUPI:      "imsi-001010000000001",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI:    models.SNSSAI{SST: 1, SD: "010203"},
	})

	request := httptest.NewRequest(http.MethodPost, "/pdu-sessions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}
}

func TestCreatePDUSessionRejectsUnregisteredUE(t *testing.T) {
	amfServer := amf.NewServer("amf-001")
	amfHTTP := httptest.NewServer(amfServer.Handler())
	defer amfHTTP.Close()

	smfServer := NewServer("smf-001", NewHTTPAMFClient(amfHTTP.URL, nil))
	body := mustJSON(t, models.PDUSessionRequest{
		SUPI:      "imsi-001010000000099",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI:    models.SNSSAI{SST: 1, SD: "010203"},
	})

	request := httptest.NewRequest(http.MethodPost, "/pdu-sessions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, response.Code)
	}
}

func TestCreatePDUSessionRejectsDeregisteredUE(t *testing.T) {
	amfServer := amf.NewServer("amf-001")
	amfHTTP := httptest.NewServer(amfServer.Handler())
	defer amfHTTP.Close()

	registerUE(t, amfHTTP.URL)
	req, err := http.NewRequest(http.MethodDelete, amfHTTP.URL+"/ues/imsi-001010000000001", nil)
	if err != nil {
		t.Fatalf("build deregistration request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deregister ue: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected deregistration status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	smfServer := NewServer("smf-001", NewHTTPAMFClient(amfHTTP.URL, nil))
	body := mustJSON(t, models.PDUSessionRequest{
		SUPI:      "imsi-001010000000001",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI:    models.SNSSAI{SST: 1, SD: "010203"},
	})

	request := httptest.NewRequest(http.MethodPost, "/pdu-sessions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, response.Code)
	}
}

func TestPDUSessionCollectionListsAndResetsState(t *testing.T) {
	amfServer := amf.NewServer("amf-001")
	amfHTTP := httptest.NewServer(amfServer.Handler())
	defer amfHTTP.Close()

	registerUE(t, amfHTTP.URL)

	smfServer := NewServer("smf-001", NewHTTPAMFClient(amfHTTP.URL, nil))
	createPDUSession(t, smfServer, 10)
	createPDUSession(t, smfServer, 11)

	list := httptest.NewRequest(http.MethodGet, "/pdu-sessions", nil)
	listResponse := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listResponse.Code)
	}

	var payload SessionListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&payload); err != nil {
		t.Fatalf("decode session list: %v", err)
	}
	if payload.Count != 2 {
		t.Fatalf("expected 2 sessions, got %d", payload.Count)
	}
	if payload.Items[0].SessionID != 10 || payload.Items[1].SessionID != 11 {
		t.Fatalf("expected sorted session ids 10 and 11, got %+v", payload.Items)
	}

	reset := httptest.NewRequest(http.MethodDelete, "/pdu-sessions", nil)
	resetResponse := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(resetResponse, reset)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resetResponse.Code)
	}

	listAfterReset := httptest.NewRequest(http.MethodGet, "/pdu-sessions", nil)
	listAfterResetResponse := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(listAfterResetResponse, listAfterReset)

	var afterReset SessionListResponse
	if err := json.NewDecoder(listAfterResetResponse.Body).Decode(&afterReset); err != nil {
		t.Fatalf("decode session list after reset: %v", err)
	}
	if afterReset.Count != 0 {
		t.Fatalf("expected empty session list, got %d", afterReset.Count)
	}
}

func createPDUSession(t *testing.T, smfServer *Server, sessionID int) {
	t.Helper()
	body := mustJSON(t, models.PDUSessionRequest{
		SUPI:      "imsi-001010000000001",
		SessionID: sessionID,
		DNN:       "internet",
		SNSSAI:    models.SNSSAI{SST: 1, SD: "010203"},
	})
	request := httptest.NewRequest(http.MethodPost, "/pdu-sessions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	smfServer.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}
}

func registerUE(t *testing.T, baseURL string) {
	t.Helper()
	body := mustJSON(t, models.RegistrationRequest{
		SUPI:       "imsi-001010000000001",
		PLMNID:     "00101",
		AccessType: "3GPP_ACCESS",
	})
	resp, err := http.Post(baseURL+"/registration", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register ue: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected registration status %d, got %d", http.StatusCreated, resp.StatusCode)
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
