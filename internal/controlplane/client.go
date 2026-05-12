package controlplane

import "context"

type Client interface {
	Register(context.Context, RegistrationRequest) (Response, error)
	CreatePDUSession(context.Context, PDUSessionRequest) (Response, error)
}

type RegistrationRequest struct {
	IMSI  string `json:"imsi"`
	SUPI  string `json:"supi"`
	GNBID string `json:"gnb_id"`
}

type PDUSessionRequest struct {
	IMSI      string `json:"imsi"`
	SUPI      string `json:"supi"`
	SessionID int    `json:"session_id"`
	DNN       string `json:"dnn"`
	SNSSAI    SNSSAI `json:"s_nssai"`
}

type SNSSAI struct {
	SST int    `json:"sst"`
	SD  string `json:"sd"`
}

type Response struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	SessionRef string `json:"session_ref,omitempty"`
}
