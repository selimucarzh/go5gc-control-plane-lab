package sim

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"go5gc-control-plane-lab/internal/config"
	"go5gc-control-plane-lab/internal/controlplane"
)

func TestSimulatorRun(t *testing.T) {
	cfg := config.Config{
		Mode:      config.ModeMock,
		Count:     1,
		MCC:       "001",
		MNC:       "01",
		StartMSIN: 1,
		GNBID:     "gNB-001",
		DNN:       "internet",
		SessionID: 10,
		SNSSAI: config.SNSSAI{
			SST: 1,
			SD:  "010203",
		},
		Timeout: 5 * time.Second,
	}

	var logs bytes.Buffer
	logger := log.New(&logs, "", 0)

	simulator := NewSimulator(controlplane.NewMockClient(), logger, cfg)
	if err := simulator.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := logs.String()
	for _, expected := range []string{
		`"event":"ue.created"`,
		`"event":"registration.response"`,
		`"event":"pdu-session.response"`,
		`"supi":"imsi-001010000000001"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %s, got %s", expected, output)
		}
	}
}
