package wmshttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	receivingports "github.com/company/pda-backend/internal/execution/receiving/ports"
	"github.com/company/pda-backend/internal/integration/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
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

func TestOwnerResponseErrorUsesHTTPStatusForHumanErrorText(t *testing.T) {
	err := ownerResponseError(http.StatusNotFound, []byte(`{"error":"Not Found"}`))
	var domainErr *platform.DomainError
	if !errors.As(err, &domainErr) {
		t.Fatalf("expected domain error, got %T", err)
	}
	if domainErr.Code != "UPSTREAM_NOT_FOUND" {
		t.Fatalf("code: got %q, want UPSTREAM_NOT_FOUND", domainErr.Code)
	}
}

func TestWarehouseAliasesAreExplicitAndCaseInsensitive(t *testing.T) {
	aliases := parseWarehouseAliases("MAIN=warehouse-uuid, invalid, OTHER=other-uuid")
	if aliases["MAIN"] != "warehouse-uuid" || aliases["OTHER"] != "other-uuid" {
		t.Fatalf("unexpected aliases: %#v", aliases)
	}
	if _, ok := aliases["INVALID"]; ok {
		t.Fatal("malformed alias was accepted")
	}
}

func TestWarehouseMatchesAllConfiguredDemoContext(t *testing.T) {
	t.Setenv("PDA_UPSTREAM_WMS_WAREHOUSE_ALIASES", "MAIN=*")
	client, err := New("http://wms.test", "test-token", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !client.WarehouseMatches(ports.Location{WarehouseID: "fg-warehouse", WarehouseCode: "WH-KZ3-FG"}, "MAIN") {
		t.Fatal("MAIN demo context should include an authorized warehouse location")
	}
	if got, err := client.CanonicalWarehouseID(context.Background(), "MAIN"); err != nil || got != "" {
		t.Fatalf("MAIN wildcard should not be sent as a physical warehouse filter: got=%q err=%v", got, err)
	}
}

func TestListInventoryBalancesAcceptsOwnerArrayResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != inventoryPath+"/balances" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"lot_id":"lot-1","lot_code":"LOT-1","item_revision_id":"item-1","uom_code":"KG","location_id":"loc-1","on_hand_qty":12.5,"reserved_qty":2,"available_qty":10.5,"status":"Active","row_version":3}]`))
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.ListInventoryBalances(context.Background(), "item-1", "loc-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].LotCode != "LOT-1" || rows[0].OnHandQty != 12.5 || rows[0].RowVersion != 3 {
		t.Fatalf("unexpected inventory balances: %+v", rows)
	}
}

func TestReceivingAndLocationAdaptersMapOwnerContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == inboundReceiptsPath && r.Method == http.MethodGet {
			if r.URL.Query().Get("warehouse_location_id") != "loc-1" {
				t.Fatalf("missing location scope: %s", r.URL.String())
			}
			_, _ = w.Write([]byte(`{"data":[{"receipt_id":"r-1","receipt_code":"R-1","warehouse_location_id":"loc-1","status":"Draft","line_count":1}]}`))
			return
		}
		if r.URL.Path == locationPath+"/loc-1" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"location_id":"loc-1","location_code":"RECV-01","location_name":{"vi":"Khu chờ nhập","en":"Receiving staging"},"warehouse_id":"wh-1","warehouse_code":"MAIN"}`))
			return
		}
		if r.URL.Path == inboundReceiptsPath+"/r-1/confirm" && r.Method == http.MethodPost {
			if r.Header.Get("Idempotency-Key") != "confirm-1" {
				t.Fatalf("missing idempotency key")
			}
			_, _ = w.Write([]byte(`{"receipt_id":"r-1","status":"Confirmed","confirmation_status":"CONFIRMED","line_count":1,"result":"CREATED"}`))
			return
		}
		if r.URL.Path == inboundReceiptsPath+"/r-1/lines/l-1/quantity" && r.Method == http.MethodPost {
			if r.Header.Get("Idempotency-Key") != "quantity-1" {
				t.Fatalf("missing quantity idempotency key")
			}
			_, _ = w.Write([]byte(`{"command_id":"c-1","receipt_id":"r-1","line_id":"l-1","status":"RECORDED","actual_quantity":2,"uom_code":"EA","version":2}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	receipts, err := client.ListReceipts(context.Background(), ports.ReceiptQuery{WarehouseLocationID: "loc-1", Limit: 20})
	if err != nil || len(receipts) != 1 || receipts[0].ReceiptID != "r-1" {
		t.Fatalf("receipts=%+v err=%v", receipts, err)
	}
	location, err := client.Location(context.Background(), "loc-1")
	if err != nil || location.WarehouseID != "wh-1" || location.WarehouseCode != "MAIN" || location.Name != "Khu chờ nhập" {
		t.Fatalf("location=%+v err=%v", location, err)
	}
	confirmed, err := client.ConfirmReceipt(context.Background(), "r-1", "confirm-1")
	if err != nil || confirmed.Status != "Confirmed" {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	quantity, err := client.RecordReceiptQuantity(context.Background(), "r-1", "l-1", ports.ReceiptQuantityRequest{ActualQuantity: 2, UOMCode: "EA", ExpectedVersion: 1, ItemRevisionID: "item-1", LotCode: "lot-1", IdempotencyKey: "quantity-1"})
	if err != nil || quantity.Status != "RECORDED" || quantity.Version != 2 {
		t.Fatalf("quantity=%+v err=%v", quantity, err)
	}
}

func TestExecutionOperatorRolePropagatesForScanAndConfirm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Role-Code") != "WMS_EXECUTION_OPERATOR" {
			t.Fatalf("missing execution operator role: %q", r.Header.Get("X-Role-Code"))
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/scans") {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/confirm") {
			_, _ = w.Write([]byte(`{"task":{"task_id":"task-1","task_type":"PUTAWAY","status":"COMPLETED","warehouse_id":"wh-1","version":2}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.RecordExecutionScan(context.Background(), "task-1", "SOURCE", "BIN-01", 1, "operator-1", "trace-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ApplyExecutionTaskCommand(context.Background(), "task-1", executionTaskCommand{CommandID: "command-1", CommandType: "CONFIRM", ExpectedVersion: 1}, "operator-1", "idempotency-1"); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionGroupConfirmUsesHyphenatedOwnerRoute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != executionTasksPath+"/task-1/confirm-group" {
			t.Fatalf("path = %q, want confirm-group route", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task":{"task_id":"task-1","task_type":"PUTAWAY","status":"COMPLETED","warehouse_id":"wh-1","version":9}}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ApplyExecutionTaskCommand(context.Background(), "task-1", executionTaskCommand{CommandID: "command-1", CommandType: "CONFIRM_GROUP", ExpectedVersion: 8}, "operator-1", "idempotency-1"); err != nil {
		t.Fatal(err)
	}
}

func TestReceivingListReturnsOwnerLineSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == inboundReceiptsPath:
			_, _ = w.Write([]byte(`{"data":[{"receipt_id":"r-2","receipt_code":"R-2","warehouse_location_id":"loc-2","status":"Draft","assigned_operator_id":"op-1","assignment_status":"CLAIMED","line_count":2}]}`))
		case r.Method == http.MethodGet && r.URL.Path == locationPath+"/loc-2":
			_, _ = w.Write([]byte(`{"location_id":"loc-2","location_code":"RECV-02","warehouse_id":"wh-1","warehouse_code":"MAIN"}`))
		case r.Method == http.MethodGet && r.URL.Path == inboundReceiptsPath+"/r-2":
			_, _ = w.Write([]byte(`{"receipt_id":"r-2","receipt_code":"R-2","warehouse_location_id":"loc-2","status":"Draft","assigned_operator_id":"op-1","assignment_status":"CLAIMED","assignment_version":1,"lines":[{"line_id":"line-a","item_revision_id":"item-a","lot_code":"LOT-A","expected_quantity":2,"received_quantity":0},{"line_id":"line-b","item_revision_id":"item-b","lot_code":"LOT-B","expected_quantity":3,"received_quantity":0}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewReceivingAdapter(client)
	page, err := adapter.List(context.Background(), receivingports.Filter{WarehouseID: "wh-1", OperatorID: "op-1", Limit: 20}, platform.ActorContext{OperatorID: "op-1", WarehouseID: "wh-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || len(page.Items[0].Lines) != 2 || page.Items[0].Lines[0].Barcode != "LOT-A" || page.Items[0].Lines[1].ItemID != "item-b" {
		t.Fatalf("receiving list lost owner line snapshot: %+v", page.Items)
	}
}

func TestReceivingListAcceptsLocalizedItemNameSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == inboundReceiptsPath:
			_, _ = w.Write([]byte(`{"data":[{"receipt_id":"r-localized","receipt_code":"RCV-LOCALIZED","warehouse_location_id":"loc-1","status":"Draft","assigned_operator_id":"op-1","assignment_status":"CLAIMED","lines":[{"line_id":"line-1","item_revision_id":"item-1","item_name":{"vi":"Cao su carbon", "en":"Carbon black"},"lot_code":"LOT-1","expected_quantity":3}]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == locationPath+"/loc-1":
			_, _ = w.Write([]byte(`{"location_id":"loc-1","warehouse_id":"wh-1","warehouse_code":"MAIN"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	page, err := NewReceivingAdapter(client).List(context.Background(), receivingports.Filter{WarehouseID: "wh-1", OperatorID: "op-1", Limit: 20}, platform.ActorContext{OperatorID: "op-1", WarehouseID: "wh-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || len(page.Items[0].Lines) != 1 || page.Items[0].Lines[0].ItemName != "Cao su carbon" {
		t.Fatalf("localized item name was not mapped: %+v", page.Items)
	}
}

func TestReceivingBarcodeResolvesLPNBeforeItemResolver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == inboundReceiptsPath+"/receipt-lpn":
			_, _ = w.Write([]byte(`{"receipt_id":"receipt-lpn","receipt_code":"RCV-1","lpn_code":"LPN-IN-1","warehouse_location_id":"loc-1","status":"Draft","assignment_status":"CLAIMED","assigned_operator_id":"op-1","assignment_version":1,"lines":[{"line_id":"line-1","item_revision_id":"item-1","lot_code":"LOT-1","expected_quantity":5,"received_quantity":0}]}`))
		case r.Method == http.MethodGet && r.URL.Path == locationPath+"/loc-1":
			_, _ = w.Write([]byte(`{"location_id":"loc-1","warehouse_id":"wh-1","warehouse_code":"MAIN"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	line, err := NewReceivingAdapter(client).ResolveBarcodeWithContext(
		context.Background(), "receipt-lpn", "LPN-IN-1", "CODE128", "RECEIVING_LPN",
		platform.ActorContext{OperatorID: "op-1", WarehouseID: "wh-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !line.ReceiptVerified || line.ID != "line-1" {
		t.Fatalf("LPN resolution = %+v, want verified line-1", line)
	}
}

func TestReceivingListKeepsCompletedReceiptForHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == inboundReceiptsPath {
			_, _ = w.Write([]byte(`{"data":[{"receipt_id":"r-completed","receipt_code":"R-COMP","warehouse_location_id":"loc-1","status":"Confirmed","confirmation_status":"CONFIRMED","assigned_operator_id":"op-1","assignment_status":"COMPLETED","assignment_version":2,"lines":[{"line_id":"line-1","item_revision_id":"item-1","lot_code":"LOT-1","expected_quantity":4,"received_quantity":4}]}]}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == locationPath+"/loc-1" {
			_, _ = w.Write([]byte(`{"location_id":"loc-1","warehouse_id":"wh-1","warehouse_code":"MAIN"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	page, err := NewReceivingAdapter(client).List(context.Background(), receivingports.Filter{WarehouseID: "wh-1", OperatorID: "op-1", Limit: 20}, platform.ActorContext{OperatorID: "op-1", WarehouseID: "wh-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != receivingdomain.StatusCompleted {
		t.Fatalf("completed receipt was not retained for history: %+v", page.Items)
	}
}

func TestWarehouseMatchesCanonicalIDOrBusinessCode(t *testing.T) {
	location := ports.Location{WarehouseID: "warehouse-uuid", WarehouseCode: "MAIN"}
	for _, warehouse := range []string{"warehouse-uuid", "MAIN", "main"} {
		if !warehouseMatches(location, warehouse) {
			t.Fatalf("warehouse %q should match %+v", warehouse, location)
		}
	}
	if warehouseMatches(location, "OTHER") {
		t.Fatal("different warehouse matched")
	}
}

func TestResolveBarcodeMapsMasterDataContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != barcodeResolvePath {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Calling-Service") != "PDA_BACKEND" || r.Header.Get("Idempotency-Key") != "scan-1" {
			t.Fatalf("missing service/idempotency headers")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		for key, expected := range map[string]string{
			"raw_value": "ITEM-001", "symbology": "CODE128", "scan_context": "RECEIVING_ITEM",
			"warehouse_id": "11111111-1111-4111-8111-111111111111", "task_id": "22222222-2222-4222-8222-222222222222", "line_id": "33333333-3333-4333-8333-333333333333", "scan_id": "scan-1",
		} {
			if body[key] != expected {
				t.Fatalf("body[%s]=%v, want %s", key, body[key], expected)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"RESOLVED","normalized_value":"ITEM-001","item_revision_id":"44444444-4444-4444-8444-444444444444","uom_code":"EA"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := client.ResolveBarcode(context.Background(), "ITEM-001", "CODE128", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333", "scan-1")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ItemID != "44444444-4444-4444-8444-444444444444" || resolved.UOM != "EA" || resolved.NormalizedValue != "ITEM-001" {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func TestConfirmShippingUsesOwnerStoredFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != shippingPath+"/shipments/shipment-1/confirm" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["command_id"] != "command-1" || body["expected_version"] != float64(4) {
			t.Fatalf("unexpected owner command: %#v", body)
		}
		if _, exists := body["carrier"]; exists {
			t.Fatal("carrier must remain authoritative in Shipping")
		}
		if _, exists := body["tracking_number"]; exists {
			t.Fatal("tracking must remain authoritative in Shipping")
		}
		if r.Header.Get("Idempotency-Key") != "key-1" || r.Header.Get("X-Calling-Service") != "PDA_BACKEND" {
			t.Fatal("missing command headers")
		}
		_, _ = w.Write([]byte(`{"status":"ACKNOWLEDGED","version":5}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ConfirmShipping(context.Background(), "shipment-1", "command-1", 4, "key-1")
	if err != nil || result["status"] != "ACKNOWLEDGED" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAllocateExecutionTaskMapsOwnerCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != executionTasksPath+"/task-1/allocate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["command_id"] != "command-1" || r.Header.Get("X-User-ID") != "operator-1" || r.Header.Get("X-Trace-ID") != "trace-1" || r.Header.Get("Idempotency-Key") != "key-1" {
			t.Fatalf("allocation context missing: body=%#v headers=%v", body, r.Header)
		}
		_, _ = w.Write([]byte(`{"task_id":"task-1","task_type":"PICKING","warehouse_id":"wh-1","status":"ALLOCATED","version":2,"details":{"qty":4}}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	task, err := client.AllocateExecutionTask(context.Background(), "task-1", "command-1", "trace-1", "operator-1", "key-1")
	if err != nil || task.TaskID != "task-1" || task.Status != "ALLOCATED" || task.Version != 2 {
		t.Fatalf("task=%+v err=%v", task, err)
	}
}

func TestCycleCountClientMapsOwnerReadAndSubmitContracts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == cycleCountsPath {
			if r.URL.Query().Get("warehouse_id") != "11111111-1111-4111-8111-111111111111" {
				t.Fatalf("missing warehouse scope")
			}
			_, _ = w.Write([]byte(`[{"count_task_id":"55555555-5555-4555-8555-555555555555","warehouse_id":"11111111-1111-4111-8111-111111111111","location_id":"66666666-6666-4666-8666-666666666666","status":"OPEN","version":1,"lines":[{"count_line_id":"77777777-7777-4777-8777-777777777777","item_revision_id":"88888888-8888-4888-8888-888888888888","snapshot_quantity":10,"status":"PENDING","version":1}]}]`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == cycleCountsPath+"/55555555-5555-4555-8555-555555555555" {
			_, _ = w.Write([]byte(`{"count_task_id":"55555555-5555-4555-8555-555555555555","warehouse_id":"11111111-1111-4111-8111-111111111111","location_id":"66666666-6666-4666-8666-666666666666","status":"SUBMITTED","version":2,"lines":[{"count_line_id":"77777777-7777-4777-8777-777777777777","item_revision_id":"88888888-8888-4888-8888-888888888888","snapshot_quantity":10,"submitted_quantity":8,"variance_quantity":-2,"status":"PENDING_APPROVAL","version":2}]}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == cycleCountsPath+"/55555555-5555-4555-8555-555555555555/lines/77777777-7777-4777-8777-777777777777/submit" {
			if r.Header.Get("Idempotency-Key") != "count-command-1" || r.Header.Get("X-User-ID") != "operator-1" {
				t.Fatalf("missing count command context")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":"CREATED"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := client.ListCycleCounts(context.Background(), "11111111-1111-4111-8111-111111111111", "operator-1", "")
	if err != nil || len(rows) != 1 || rows[0].Lines[0].ItemID == "" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	row, err := client.GetCycleCount(context.Background(), "55555555-5555-4555-8555-555555555555", "11111111-1111-4111-8111-111111111111")
	if err != nil || row.Status != "SUBMITTED" || row.Lines[0].SubmittedQuantity == nil {
		t.Fatalf("row=%+v err=%v", row, err)
	}
	if err := client.SubmitCycleCount(context.Background(), "55555555-5555-4555-8555-555555555555", "77777777-7777-4777-8777-777777777777", 8, 1, "count-command-1", "device-1", "operator-1"); err != nil {
		t.Fatal(err)
	}
}

func TestCycleCountLifecycleCommandsPropagateVersionAndIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || (r.URL.Path != cycleCountsPath+"/task-1/lines/line-1/recount" && r.URL.Path != cycleCountsPath+"/task-1/complete") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["command_id"] != "command-1" || body["expected_version"] != float64(3) || body["idempotency_key"] != "key-1" {
			t.Fatalf("unexpected count command: %#v", body)
		}
		_, _ = w.Write([]byte(`{"status":"RECOUNT_REQUIRED"}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.RecountCycleCount(context.Background(), "task-1", "line-1", "command-1", 3, "key-1"); err != nil {
		t.Fatal(err)
	}
	if err := client.CompleteCycleCount(context.Background(), "task-1", "command-1", 3, "key-1"); err != nil {
		t.Fatal(err)
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

func TestConfirmReceiptPreservesAcceptedOwnerBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"error":"OVER_RECEIPT_APPROVAL_REQUIRED","message":"approval required"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ConfirmReceipt(context.Background(), "receipt-1", "command-1")
	if err == nil || !strings.Contains(err.Error(), "OVER_RECEIPT_APPROVAL_REQUIRED") {
		t.Fatalf("expected owner business error, got %v", err)
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
