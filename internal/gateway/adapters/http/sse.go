package httpadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	executionports "github.com/company/pda-backend/internal/execution/ports"
)

// taskEvents is an operator-scoped invalidation stream. REST remains the
// source of truth; clients refetch tasks after receiving TASK_UPDATED.
func (r *Router) taskEvents(w http.ResponseWriter, req *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE_UNSUPPORTED", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Encoding", "identity")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	// Send a real data frame immediately. Some reverse proxies buffer comment
	// frames until a later heartbeat, which makes a newly assigned task appear
	// missing even though the stream is already connected.
	_, _ = fmt.Fprint(w, "event: READY\ndata: {\"status\":\"connected\"}\n\n")
	// Some managed tunnels buffer small chunked responses even when the
	// origin flushes. Keep the padding bounded, but large enough to cross
	// those proxy thresholds so the READY/task frames reach the device.
	_, _ = fmt.Fprintf(w, ": %s\n\n", strings.Repeat(" ", 16*1024))
	flusher.Flush()

	actor := r.actor(req)
	r.logger.Info("sse_connected", "operatorId", actor.OperatorID, "warehouseId", actor.WarehouseID, "lastEventId", req.Header.Get("Last-Event-ID"), "correlationId", correlation(req.Context()))
	limit := 100
	lastDigest := ""
	poll := time.NewTicker(1 * time.Second)
	defer poll.Stop()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()

	emitTasks := func() {
		page, err := r.tasks.List(req.Context(), executionports.TaskFilter{Limit: limit}, actor)
		if err != nil {
			r.logger.Warn("sse_task_source_unavailable", "operatorId", actor.OperatorID, "warehouseId", actor.WarehouseID, "error", err, "correlationId", correlation(req.Context()))
			_, _ = fmt.Fprint(w, "event: RESYNC_REQUIRED\ndata: {\"reason\":\"TASK_SOURCE_UNAVAILABLE\"}\n\n")
			flusher.Flush()
			return
		}
		digest := taskDigest(page)
		if digest == lastDigest {
			return
		}
		lastDigest = digest
		r.logger.Info("sse_task_snapshot", "operatorId", actor.OperatorID, "warehouseId", actor.WarehouseID, "taskCount", len(page.Tasks), "digest", digest[:16], "correlationId", correlation(req.Context()))
		for _, task := range page.Tasks {
			data, _ := json.Marshal(map[string]any{
				"eventId":   fmt.Sprintf("task:%s:%s", task.ID, digest[:16]),
				"eventType": "TASK_UPDATED", "aggregateType": task.Category,
				"aggregateId": task.ID, "aggregateVersion": task.Version,
				"operatorId": task.OperatorID, "data": task,
			})
			_, _ = fmt.Fprintf(w, "id: task:%s:%s\nevent: TASK_UPDATED\ndata: %s\n\n", task.ID, digest[:16], data)
			flusher.Flush()
		}
	}

	// Do not make a newly opened stream wait for the first ticker interval.
	emitTasks()

	for {
		select {
		case <-req.Context().Done():
			r.logger.Info("sse_disconnected", "operatorId", actor.OperatorID, "warehouseId", actor.WarehouseID, "correlationId", correlation(req.Context()))
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, "event: HEARTBEAT\ndata: {\"status\":\"alive\"}\n\n")
			flusher.Flush()
		case <-poll.C:
			emitTasks()
		}
	}
}

func taskDigest(page executionports.TaskPage) string {
	data, _ := json.Marshal(page.Tasks)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
