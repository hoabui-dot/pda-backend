package wmshttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	platform "github.com/company/pda-backend/internal/platform/domain"
)

const finishedProductTracePath = "/api/wms/inbound/finished-products"

// Trace reads the WMS-owned finished-product trace projection. It is
// intentionally read-only; PDA Backend does not resolve inventory or MES
// state itself.
func (c *Client) Trace(ctx context.Context, barcode string, actor platform.ActorContext) (map[string]any, error) {
	if barcode == "" || len(barcode) > 160 {
		return nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Finished product barcode is invalid"}
	}
	payload, err := c.getJSONBytes(ctx, finishedProductTracePath+"/"+url.PathEscape(barcode)+"/trace")
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.Data == nil {
		return nil, fmt.Errorf("decode WMS finished product trace response: %w", err)
	}
	return envelope.Data, nil
}
