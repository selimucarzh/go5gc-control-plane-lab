package models

// RegistrationRequest is the first message sent by a UE to the AMF.
type RegistrationRequest struct {
	SUPI       string `json:"supi"`
	GUTI       string `json:"guti,omitempty"`
	PLMNID     string `json:"plmn_id"`
	AccessType string `json:"access_type"`
}

// RegistrationResponse is returned after the AMF stores UE context.
type RegistrationResponse struct {
	Accepted bool   `json:"accepted"`
	AMFID    string `json:"amf_id"`
	UEID     string `json:"ue_id"`
	Message  string `json:"message"`
}
