package models

type PDUSessionRequest struct {
	SUPI      string `json:"supi"`
	SessionID int    `json:"session_id"`
	DNN       string `json:"dnn"`
	SNSSAI    SNSSAI `json:"s_nssai"`
}

type SNSSAI struct {
	SST int    `json:"sst"`
	SD  string `json:"sd"`
}

type PDUSessionResponse struct {
	Accepted   bool   `json:"accepted"`
	SMFID      string `json:"smf_id"`
	SessionRef string `json:"session_ref"`
	Message    string `json:"message"`
}
