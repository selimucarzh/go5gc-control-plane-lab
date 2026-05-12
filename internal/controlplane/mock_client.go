package controlplane

import (
	"context"
	"fmt"
)

type MockClient struct{}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (c *MockClient) Register(_ context.Context, req RegistrationRequest) (Response, error) {
	return Response{
		Status:  "registered",
		Message: fmt.Sprintf("UE %s registered on %s", req.SUPI, req.GNBID),
	}, nil
}

func (c *MockClient) CreatePDUSession(_ context.Context, req PDUSessionRequest) (Response, error) {
	return Response{
		Status:     "pdu-session-created",
		Message:    fmt.Sprintf("PDU session %d created for %s on DNN %s", req.SessionID, req.SUPI, req.DNN),
		SessionRef: fmt.Sprintf("%s-%d", req.SUPI, req.SessionID),
	}, nil
}
