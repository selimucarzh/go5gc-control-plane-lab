package cpstub

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go5gc-control-plane-lab/internal/controlplane"
)

func TestServerEndToEnd(t *testing.T) {
	var logs bytes.Buffer
	server := NewServer(log.New(&logs, "", 0))
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	client := controlplane.NewHTTPClient(httpServer.URL, 2*time.Second)

	registrationResponse, err := client.Register(context.Background(), controlplane.RegistrationRequest{
		IMSI:  "001010000000001",
		SUPI:  "imsi-001010000000001",
		GNBID: "gNB-001",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if got, want := registrationResponse.Status, "registered"; got != want {
		t.Fatalf("registration status mismatch: got %s want %s", got, want)
	}

	pduResponse, err := client.CreatePDUSession(context.Background(), controlplane.PDUSessionRequest{
		IMSI:      "001010000000001",
		SUPI:      "imsi-001010000000001",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI: controlplane.SNSSAI{
			SST: 1,
			SD:  "010203",
		},
	})
	if err != nil {
		t.Fatalf("CreatePDUSession returned error: %v", err)
	}
	if got, want := pduResponse.Status, "pdu-session-created"; got != want {
		t.Fatalf("pdu session status mismatch: got %s want %s", got, want)
	}

	output := logs.String()
	for _, expected := range []string{
		"event=registration",
		"event=pdu-session",
		"imsi-001010000000001-10",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %s, got %s", expected, output)
		}
	}
}

func TestPDUSessionRequiresRegistration(t *testing.T) {
	server := NewServer(log.New(&bytes.Buffer{}, "", 0))
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	reqBody, err := json.Marshal(controlplane.PDUSessionRequest{
		IMSI:      "001010000000099",
		SUPI:      "imsi-001010000000099",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI: controlplane.SNSSAI{
			SST: 1,
			SD:  "010203",
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	response, err := http.Post(httpServer.URL+"/pdu-sessions", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST returned error: %v", err)
	}
	defer response.Body.Close()

	if got, want := response.StatusCode, http.StatusConflict; got != want {
		t.Fatalf("status code mismatch: got %d want %d", got, want)
	}

	var decoded controlplane.Response
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if got, want := decoded.Status, "rejected"; got != want {
		t.Fatalf("response status mismatch: got %s want %s", got, want)
	}
}
