package wmshttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWarehousesMapsApprovedWMSResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != warehousePath || r.URL.Query().Get("limit") != "500" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"warehouse_id":"wh-1","warehouse_code":"WH-01","warehouse_name":{"vi":"Kho 1","en":"Warehouse 1"}}]}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	warehouses, err := client.Warehouses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(warehouses) != 1 || warehouses[0].ID != "wh-1" || warehouses[0].Code != "WH-01" || warehouses[0].Name != "Kho 1" {
		t.Fatalf("unexpected mapped warehouses: %+v", warehouses)
	}
}

func TestNewAppliesDefaultTimeout(t *testing.T) {
	client, err := New("https://wms.example.test", "token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.http.Timeout != 10*time.Second {
		t.Fatalf("timeout=%s", client.http.Timeout)
	}
}

func TestWarehousesRejectsMalformedData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"warehouse_id":"wh-1","warehouse_code":"WH-01","warehouse_name":{}}]}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Warehouses(context.Background()); err == nil {
		t.Fatal("expected malformed warehouse name error")
	}
}

func TestWarehousesRejectsUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Warehouses(context.Background()); err == nil {
		t.Fatal("expected upstream status error")
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	if _, err := New("not-a-url", "token", nil); err == nil {
		t.Fatal("expected invalid URL error")
	}
	if _, err := New("https://wms.example.test", "", nil); err == nil {
		t.Fatal("expected missing token error")
	}
}
