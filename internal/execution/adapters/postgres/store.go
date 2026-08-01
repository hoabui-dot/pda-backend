package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}
type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }
func (s *Store) Seed(ctx context.Context, tasks []domain.Task) error {
	for _, t := range tasks {
		_, err := s.pool.Exec(ctx, `INSERT INTO warehouse_task(task_id,category,status,priority,warehouse_id,operator_id,version,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(task_id) DO NOTHING`, t.ID, t.Category, t.Status, t.Priority, t.WarehouseID, t.OperatorID, t.Version, t.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) InvalidateTaskViews(context.Context, string, string) error { return nil }
func (s *Store) db(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return s.pool
}
func (s *Store) WithinTransaction(ctx context.Context, operation func(context.Context) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := operation(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) GetForUpdate(ctx context.Context, id string) (domain.Task, error) {
	return scanTask(s.db(ctx).QueryRow(ctx, `SELECT task_id,category,status,priority,warehouse_id,operator_id,version,updated_at FROM warehouse_task WHERE task_id=$1 FOR UPDATE`, id))
}
func (s *Store) Save(ctx context.Context, t domain.Task) error {
	tag, err := s.db(ctx).Exec(ctx, `UPDATE warehouse_task SET status=$2,operator_id=$3,version=$4,updated_at=$5 WHERE task_id=$1`, t.ID, t.Status, t.OperatorID, t.Version, t.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrTaskNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context, f ports.TaskFilter) (ports.TaskPage, error) {
	after := ""
	if f.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return ports.TaskPage{}, fmt.Errorf("invalid cursor")
		}
		after = string(decoded)
	}
	rows, err := s.db(ctx).Query(ctx, `SELECT task_id,category,status,priority,warehouse_id,operator_id,version,updated_at FROM warehouse_task WHERE warehouse_id=$1 AND (operator_id IS NULL OR operator_id=$2) AND ($3='' OR category=$3) AND ($4='' OR status=$4) AND ($5='' OR task_id ILIKE '%'||$5||'%') AND task_id>$6 ORDER BY task_id LIMIT $7`, f.WarehouseID, f.OperatorID, f.Category, f.Status, f.Query, after, f.Limit+1)
	if err != nil {
		return ports.TaskPage{}, err
	}
	defer rows.Close()
	items := []domain.Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return ports.TaskPage{}, err
		}
		items = append(items, task)
	}
	page := ports.TaskPage{}
	if len(items) > f.Limit {
		cursor := base64.RawURLEncoding.EncodeToString([]byte(items[f.Limit-1].ID))
		page.NextCursor = &cursor
		items = items[:f.Limit]
	}
	page.Tasks = items
	return page, rows.Err()
}
func (s *Store) Summary(ctx context.Context, warehouseID, operatorID, status string) ([]ports.SummaryItem, error) {
	rows, err := s.db(ctx).Query(ctx, `SELECT category,status,count(*) FROM warehouse_task WHERE warehouse_id=$1 AND (operator_id IS NULL OR operator_id=$2) AND ($3='' OR status=$3) GROUP BY category,status ORDER BY category,status`, warehouseID, operatorID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ports.SummaryItem{}
	for rows.Next() {
		var item ports.SummaryItem
		if err := rows.Scan(&item.Category, &item.Status, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s *Store) Dashboard(ctx context.Context, warehouseID, operatorID string) (ports.Dashboard, error) {
	var d ports.Dashboard
	err := s.db(ctx).QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE operator_id=$2 AND status='ASSIGNED'),count(*) FILTER(WHERE operator_id=$2 AND status='IN_PROGRESS'),count(*) FILTER(WHERE operator_id=$2 AND status='COMPLETED'),count(*) FILTER(WHERE priority>=80),COALESCE(max(updated_at),now()) FROM warehouse_task WHERE warehouse_id=$1 AND (operator_id IS NULL OR operator_id=$2)`, warehouseID, operatorID).Scan(&d.Total, &d.Assigned, &d.InProgress, &d.Completed, &d.HighPriority, &d.UpdatedAt)
	return d, err
}

type Idempotency struct{ Store *Store }

func (i Idempotency) Find(ctx context.Context, key string) (ports.IdempotencyResult, bool, error) {
	if _, err := i.Store.db(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return ports.IdempotencyResult{}, false, err
	}
	var data []byte
	err := i.Store.db(ctx).QueryRow(ctx, `SELECT result_json FROM command_idempotency WHERE idempotency_key=$1`, key).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.IdempotencyResult{}, false, nil
	}
	if err != nil {
		return ports.IdempotencyResult{}, false, err
	}
	var result ports.IdempotencyResult
	err = json.Unmarshal(data, &result)
	return result, true, err
}
func (i Idempotency) Save(ctx context.Context, key string, result ports.IdempotencyResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = i.Store.db(ctx).Exec(ctx, `INSERT INTO command_idempotency(idempotency_key,result_json) VALUES($1,$2)`, key, data)
	return err
}
func (s *Store) Append(ctx context.Context, envelope event.DomainEventEnvelope) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = s.db(ctx).Exec(ctx, `INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json) VALUES($1,$2,$3,$4,$5)`, envelope.EventID, envelope.AggregateID, envelope.AggregateVersion, envelope.EventType, data)
	return err
}
func scanTask(row pgx.Row) (domain.Task, error) {
	var t domain.Task
	var category, status string
	err := row.Scan(&t.ID, &category, &status, &t.Priority, &t.WarehouseID, &t.OperatorID, &t.Version, &t.UpdatedAt)
	t.Category = domain.TaskCategory(category)
	t.Status = domain.TaskStatus(status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	return t, err
}
