package memory

import (
	"context"
	"encoding/base64"
	"sort"
	"strings"
	"sync"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	"github.com/company/pda-backend/internal/platform/event"
)

type Store struct {
	mu            sync.Mutex
	tasks         map[string]domain.Task
	idempotency   map[string]ports.IdempotencyResult
	outbox        []event.DomainEventEnvelope
	invalidations int
}

func New(tasks []domain.Task) *Store {
	values := map[string]domain.Task{}
	for _, task := range tasks {
		values[task.ID] = task
	}
	return &Store{tasks: values, idempotency: map[string]ports.IdempotencyResult{}}
}
func (s *Store) WithinTransaction(ctx context.Context, operation func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return operation(ctx)
}
func (s *Store) GetForUpdate(_ context.Context, id string) (domain.Task, error) {
	task, ok := s.tasks[id]
	if !ok {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	return task, nil
}
func (s *Store) Save(_ context.Context, task domain.Task) error { s.tasks[task.ID] = task; return nil }
func (s *Store) Find(_ context.Context, key string) (ports.IdempotencyResult, bool, error) {
	v, ok := s.idempotency[key]
	return v, ok, nil
}
func (s *Store) SaveIdempotency(_ context.Context, key string, v ports.IdempotencyResult) error {
	s.idempotency[key] = v
	return nil
}

// Save implements the idempotency port; Go cannot overload task Save, so Idempotency wraps expose a distinct adapter below.
type Idempotency struct{ Store *Store }

func (i Idempotency) Find(ctx context.Context, key string) (ports.IdempotencyResult, bool, error) {
	return i.Store.Find(ctx, key)
}
func (i Idempotency) Save(ctx context.Context, key string, v ports.IdempotencyResult) error {
	return i.Store.SaveIdempotency(ctx, key, v)
}
func (s *Store) Append(_ context.Context, envelope event.DomainEventEnvelope) error {
	s.outbox = append(s.outbox, envelope)
	return nil
}
func (s *Store) InvalidateTaskViews(_ context.Context, _, _ string) error {
	s.invalidations++
	return nil
}
func (s *Store) Outbox() []event.DomainEventEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]event.DomainEventEnvelope(nil), s.outbox...)
}

func (s *Store) List(_ context.Context, f ports.TaskFilter) (ports.TaskPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]domain.Task, 0)
	for _, task := range s.tasks {
		if task.WarehouseID != f.WarehouseID {
			continue
		}
		if task.OperatorID != nil && *task.OperatorID != f.OperatorID {
			continue
		}
		if f.Category != "" && string(task.Category) != f.Category {
			continue
		}
		if f.Status != "" && string(task.Status) != f.Status {
			continue
		}
		if f.Query != "" && !strings.Contains(strings.ToLower(task.ID), strings.ToLower(f.Query)) {
			continue
		}
		values = append(values, task)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
	start := 0
	if f.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return ports.TaskPage{}, domainErrorInvalidCursor.error
		}
		for index, task := range values {
			if task.ID == string(decoded) {
				start = index + 1
				break
			}
		}
	}
	end := start + f.Limit
	if end > len(values) {
		end = len(values)
	}
	page := ports.TaskPage{Tasks: append([]domain.Task(nil), values[start:end]...)}
	if end < len(values) {
		cursor := base64.RawURLEncoding.EncodeToString([]byte(values[end-1].ID))
		page.NextCursor = &cursor
	}
	return page, nil
}

var domainErrorInvalidCursor = struct{ error }{error: &simpleError{"INVALID_CURSOR"}}

type simpleError struct{ code string }

func (e *simpleError) Error() string { return e.code }
func (s *Store) Summary(_ context.Context, warehouseID, operatorID, status string) ([]ports.SummaryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	for _, task := range s.tasks {
		if task.WarehouseID != warehouseID || (status != "" && string(task.Status) != status) {
			continue
		}
		if task.OperatorID != nil && *task.OperatorID != operatorID {
			continue
		}
		key := string(task.Category) + "|" + string(task.Status)
		counts[key]++
	}
	result := make([]ports.SummaryItem, 0, len(counts))
	for key, count := range counts {
		parts := strings.Split(key, "|")
		result = append(result, ports.SummaryItem{Category: domain.TaskCategory(parts[0]), Status: domain.TaskStatus(parts[1]), Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category == result[j].Category {
			return result[i].Status < result[j].Status
		}
		return result[i].Category < result[j].Category
	})
	return result, nil
}
func (s *Store) Dashboard(_ context.Context, warehouseID, operatorID string) (ports.Dashboard, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := ports.Dashboard{}
	for _, task := range s.tasks {
		if task.WarehouseID != warehouseID {
			continue
		}
		if task.OperatorID != nil && *task.OperatorID != operatorID {
			continue
		}
		result.Total++
		if task.Priority >= 80 {
			result.HighPriority++
		}
		if task.UpdatedAt.After(result.UpdatedAt) {
			result.UpdatedAt = task.UpdatedAt.UTC()
		}
		if task.OperatorID != nil && *task.OperatorID == operatorID {
			switch task.Status {
			case domain.StatusAssigned:
				result.Assigned++
			case domain.StatusInProgress:
				result.InProgress++
			case domain.StatusCompleted:
				result.Completed++
			}
		}
	}
	return result, nil
}
