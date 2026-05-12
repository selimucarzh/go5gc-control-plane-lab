package controlplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientRegisterAndCreatePDUSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/registration":
			_, _ = w.Write([]byte(`{"status":"registered","message":"ok"}`))
		case "/pdu-sessions":
			_, _ = w.Write([]byte(`{"status":"pdu-session-created","message":"ok","session_ref":"session-1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL, 2*time.Second)

	registrationResponse, err := client.Register(context.Background(), RegistrationRequest{
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

	pduSessionResponse, err := client.CreatePDUSession(context.Background(), PDUSessionRequest{
		IMSI:      "001010000000001",
		SUPI:      "imsi-001010000000001",
		SessionID: 10,
		DNN:       "internet",
		SNSSAI: SNSSAI{
			SST: 1,
			SD:  "010203",
		},
	})
	if err != nil {
		t.Fatalf("CreatePDUSession returned error: %v", err)
	}

	if got, want := pduSessionResponse.SessionRef, "session-1"; got != want {
		t.Fatalf("session ref mismatch: got %s want %s", got, want)
	}
}
