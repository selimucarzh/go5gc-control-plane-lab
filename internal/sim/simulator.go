package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"go5gc-control-plane-lab/internal/config"
	"go5gc-control-plane-lab/internal/controlplane"
	"go5gc-control-plane-lab/internal/identity"
)

type Simulator struct {
	client controlplane.Client
	logger *log.Logger
	cfg    config.Config
}

func NewSimulator(client controlplane.Client, logger *log.Logger, cfg config.Config) *Simulator {
	return &Simulator{
		client: client,
		logger: logger,
		cfg:    cfg,
	}
}

func (s *Simulator) Run(ctx context.Context) error {
	for i := 0; i < s.cfg.Count; i++ {
		msin := s.cfg.StartMSIN + uint64(i)
		ue := identity.Generate(i+1, s.cfg.MCC, s.cfg.MNC, msin)

		s.log("ue.created", map[string]any{
			"ue_index": ue.Index,
			"imsi":     ue.IMSI,
			"supi":     ue.SUPI,
		})

		registrationResponse, err := s.client.Register(ctx, controlplane.RegistrationRequest{
			IMSI:  ue.IMSI,
			SUPI:  ue.SUPI,
			GNBID: s.cfg.GNBID,
		})
		if err != nil {
			return fmt.Errorf("register %s: %w", ue.SUPI, err)
		}

		s.log("registration.response", map[string]any{
			"ue_index": ue.Index,
			"imsi":     ue.IMSI,
			"supi":     ue.SUPI,
			"response": registrationResponse,
		})

		pduSessionResponse, err := s.client.CreatePDUSession(ctx, controlplane.PDUSessionRequest{
			IMSI:      ue.IMSI,
			SUPI:      ue.SUPI,
			SessionID: s.cfg.SessionID,
			DNN:       s.cfg.DNN,
			SNSSAI: controlplane.SNSSAI{
				SST: s.cfg.SNSSAI.SST,
				SD:  s.cfg.SNSSAI.SD,
			},
		})
		if err != nil {
			return fmt.Errorf("create pdu session for %s: %w", ue.SUPI, err)
		}

		s.log("pdu-session.response", map[string]any{
			"ue_index": ue.Index,
			"imsi":     ue.IMSI,
			"supi":     ue.SUPI,
			"response": pduSessionResponse,
		})
	}

	return nil
}

func (s *Simulator) log(event string, payload map[string]any) {
	entry := map[string]any{
		"event": event,
	}
	for key, value := range payload {
		entry[key] = value
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		s.logger.Printf("event=%s marshal_error=%q", event, err.Error())
		return
	}

	s.logger.Println(string(encoded))
}
