package wmshttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
)

const pickingTestWarehouseID = "11111111-1111-4111-8111-111111111111"

func TestPickingStartClaimsAssignedTaskBeforeStarting(t *testing.T) {
	const taskID = "picking-1"
	const operatorID = "operator-1"

	var commands []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == executionTasksPath+"/"+taskID:
			writeExecutionTask(w, taskID, "ASSIGNED", 1, operatorID, false)
		case r.Method == http.MethodPost && r.URL.Path == executionTasksPath+"/"+taskID+"/claim":
			commands = append(commands, "CLAIM")
			writeExecutionTask(w, taskID, "CLAIMED", 2, operatorID, true)
		case r.Method == http.MethodPost && r.URL.Path == executionTasksPath+"/"+taskID+"/start":
			commands = append(commands, "START")
			writeExecutionTask(w, taskID, "IN_PROGRESS", 3, operatorID, true)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewPickingAdapter(client)
	result, err := adapter.Start(context.Background(), movementapp.Command{
		CommandID:      uuid.New(),
		IdempotencyKey: "start-picking-1",
		TaskID:         taskID,
		BaseVersion:    1,
		Actor:          platform.ActorContext{OperatorID: operatorID, WarehouseID: pickingTestWarehouseID, DeviceID: "device-1", CorrelationID: "trace-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "IN_PROGRESS" {
		t.Fatalf("status = %q, want IN_PROGRESS", result.Status)
	}
	if len(commands) != 2 || commands[0] != "CLAIM" || commands[1] != "START" {
		t.Fatalf("commands = %#v, want CLAIM then START", commands)
	}
}

func writeExecutionTask(w http.ResponseWriter, id, status string, version int64, operatorID string, envelope bool) {
	task := map[string]any{
		"task_id":              id,
		"task_type":            "PICKING",
		"status":               status,
		"warehouse_id":         pickingTestWarehouseID,
		"assigned_operator_id": operatorID,
		"version":              version,
		"details":              map[string]any{},
	}
	if envelope {
		_ = json.NewEncoder(w).Encode(map[string]any{"task": task})
		return
	}
	_ = json.NewEncoder(w).Encode(task)
}
