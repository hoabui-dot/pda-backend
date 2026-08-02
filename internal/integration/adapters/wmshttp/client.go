package wmshttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/company/pda-backend/internal/integration/ports"
)

const warehousePath = "/api/wms/master-data/warehouses"

// Client implements the approved WMS public contract without exposing WMS DTOs
// beyond this adapter boundary.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string, client *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid WMS base URL")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("WMS bearer token is required")
	}
	if client == nil {
		client = &http.Client{}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: client}, nil
}

type warehouseListResponse struct {
	Data []warehouse `json:"data"`
}

type warehouse struct {
	ID   string          `json:"warehouse_id"`
	Code string          `json:"warehouse_code"`
	Name json.RawMessage `json:"warehouse_name"`
}

func (c *Client) Warehouses(ctx context.Context) ([]ports.Warehouse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+warehousePath+"?limit=500", nil)
	if err != nil {
		return nil, fmt.Errorf("build WMS warehouse request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WMS warehouse request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("WMS warehouse request returned HTTP %d", resp.StatusCode)
	}
	var payload warehouseListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode WMS warehouse response: %w", err)
	}
	out := make([]ports.Warehouse, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Code) == "" {
			return nil, fmt.Errorf("WMS warehouse response contains an incomplete warehouse")
		}
		name, err := localizedName(item.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, ports.Warehouse{ID: item.ID, Name: name})
	}
	return out, nil
}

func localizedName(raw json.RawMessage) (string, error) {
	var localized map[string]string
	if err := json.Unmarshal(raw, &localized); err == nil {
		for _, language := range []string{"vi", "en", "ja", "ko"} {
			if value := strings.TrimSpace(localized[language]); value != "" {
				return value, nil
			}
		}
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil && strings.TrimSpace(value) != "" {
		return value, nil
	}
	return "", fmt.Errorf("WMS warehouse response contains no supported warehouse name")
}
