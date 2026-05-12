package config

import (
	"errors"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ModeMock = "mock"
	ModeHTTP = "http"
)

var hexPattern = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

type Config struct {
	Mode      string
	BaseURL   string
	Count     int
	MCC       string
	MNC       string
	StartMSIN uint64
	GNBID     string
	DNN       string
	SessionID int
	SNSSAI    SNSSAI
	Timeout   time.Duration
}

type SNSSAI struct {
	SST int
	SD  string
}

func Parse() (Config, error) {
	var (
		mode      = flag.String("mode", ModeMock, "control-plane mode: mock or http")
		baseURL   = flag.String("base-url", "http://127.0.0.1:8080", "control-plane base URL")
		count     = flag.Int("count", 1, "number of UEs to simulate")
		mcc       = flag.String("mcc", "001", "mobile country code")
		mnc       = flag.String("mnc", "01", "mobile network code")
		startMSIN = flag.Uint64("start-msin", 1, "starting MSIN value")
		gnbID     = flag.String("gnb-id", "gNB-001", "serving gNB identifier")
		dnn       = flag.String("dnn", "internet", "default data network name")
		sessionID = flag.Int("session-id", 10, "default PDU session ID")
		snssai    = flag.String("snssai", "1-010203", "S-NSSAI in sst-sd form")
		timeout   = flag.Duration("timeout", 5*time.Second, "HTTP request timeout")
	)

	flag.Parse()

	parsedSNSSAI, err := parseSNSSAI(*snssai)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Mode:      strings.ToLower(strings.TrimSpace(*mode)),
		BaseURL:   strings.TrimRight(strings.TrimSpace(*baseURL), "/"),
		Count:     *count,
		MCC:       strings.TrimSpace(*mcc),
		MNC:       strings.TrimSpace(*mnc),
		StartMSIN: *startMSIN,
		GNBID:     strings.TrimSpace(*gnbID),
		DNN:       strings.TrimSpace(*dnn),
		SessionID: *sessionID,
		SNSSAI:    parsedSNSSAI,
		Timeout:   *timeout,
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func parseSNSSAI(raw string) (SNSSAI, error) {
	parts := strings.Split(strings.TrimSpace(raw), "-")
	if len(parts) != 2 {
		return SNSSAI{}, fmt.Errorf("invalid snssai %q: expected sst-sd", raw)
	}

	var snssai SNSSAI
	if _, err := fmt.Sscanf(parts[0], "%d", &snssai.SST); err != nil {
		return SNSSAI{}, fmt.Errorf("invalid snssai sst %q", parts[0])
	}

	snssai.SD = parts[1]
	return snssai, nil
}

func validate(cfg Config) error {
	switch cfg.Mode {
	case ModeMock, ModeHTTP:
	default:
		return fmt.Errorf("invalid mode %q", cfg.Mode)
	}

	if cfg.Count < 1 {
		return errors.New("count must be greater than zero")
	}

	if len(cfg.MCC) != 3 {
		return errors.New("mcc must be 3 digits")
	}

	if len(cfg.MNC) < 2 || len(cfg.MNC) > 3 {
		return errors.New("mnc must be 2 or 3 digits")
	}

	if cfg.GNBID == "" {
		return errors.New("gnb-id is required")
	}

	if cfg.DNN == "" {
		return errors.New("dnn is required")
	}

	if cfg.SessionID < 1 {
		return errors.New("session-id must be greater than zero")
	}

	if cfg.Timeout <= 0 {
		return errors.New("timeout must be greater than zero")
	}

	if cfg.SNSSAI.SST < 0 {
		return errors.New("snssai sst must be zero or greater")
	}

	if len(cfg.SNSSAI.SD) != 6 {
		return errors.New("snssai sd must be 6 hex characters")
	}

	if !hexPattern.MatchString(cfg.SNSSAI.SD) {
		return errors.New("snssai sd must contain only hex characters")
	}

	return nil
}
