package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) Register(ctx context.Context, req RegistrationRequest) (Response, error) {
	return c.post(ctx, "/registration", req)
}

func (c *HTTPClient) CreatePDUSession(ctx context.Context, req PDUSessionRequest) (Response, error) {
	return c.post(ctx, "/pdu-sessions", req)
}

func (c *HTTPClient) post(ctx context.Context, path string, payload any) (Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return Response{}, fmt.Errorf("send request to %s: %w", path, err)
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response from %s: %w", path, err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Response{}, fmt.Errorf("%s returned %d: %s", path, response.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var decoded Response
	if len(bytes.TrimSpace(rawBody)) == 0 {
		decoded = Response{
			Status:  "accepted",
			Message: "empty response body",
		}
		return decoded, nil
	}

	if err := json.Unmarshal(rawBody, &decoded); err != nil {
		return Response{}, fmt.Errorf("decode response from %s: %w", path, err)
	}

	return decoded, nil
}
