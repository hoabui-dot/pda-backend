package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/company/pda-backend/internal/execution/receiving/domain"
	"github.com/company/pda-backend/internal/execution/receiving/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}
type db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool} }
func (s *Store) Seed(ctx context.Context, tasks []domain.Task) error {
	return s.WithinTransaction(ctx, func(tx context.Context) error {
		for _, t := range tasks {
			_, err := s.db(tx).Exec(tx, `INSERT INTO warehouse_task(task_id,category,status,priority,warehouse_id,operator_id,version,updated_at) VALUES($1,'RECEIVING',$2,50,$3,$4,$5,$6) ON CONFLICT(task_id) DO NOTHING`, t.ID, t.Status, t.WarehouseID, t.OperatorID, t.Version, t.UpdatedAt)
			if err != nil {
				return err
			}
			_, err = s.db(tx).Exec(tx, `INSERT INTO receiving_task(task_id,po_number,status,warehouse_id,operator_id,version,allow_over_receipt,require_remark_on_variance,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(task_id) DO NOTHING`, t.ID, t.PONumber, t.Status, t.WarehouseID, t.OperatorID, t.Version, t.Policy.AllowOverReceipt, t.Policy.RequireRemarkOnVariance, t.UpdatedAt)
			if err != nil {
				return err
			}
			for _, line := range t.Lines {
				_, err = s.db(tx).Exec(tx, `INSERT INTO receiving_line(line_id,task_id,item_id,item_name,barcode,expected_quantity,received_quantity) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(line_id) DO NOTHING`, line.ID, t.ID, line.ItemID, line.ItemName, line.Barcode, line.ExpectedQuantity, line.ReceivedQuantity)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}
func (s *Store) db(ctx context.Context) db {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return s.pool
}
func (s *Store) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Get(ctx context.Context, id string) (domain.Task, error) { return s.get(ctx, id, "") }
func (s *Store) GetForUpdate(ctx context.Context, id string) (domain.Task, error) {
	return s.get(ctx, id, " FOR UPDATE")
}
func (s *Store) get(ctx context.Context, id, suffix string) (domain.Task, error) {
	var t domain.Task
	var status string
	err := s.db(ctx).QueryRow(ctx, `SELECT task_id,po_number,status,warehouse_id,operator_id,version,allow_over_receipt,require_remark_on_variance,updated_at FROM receiving_task WHERE task_id=$1`+suffix, id).Scan(&t.ID, &t.PONumber, &status, &t.WarehouseID, &t.OperatorID, &t.Version, &t.Policy.AllowOverReceipt, &t.Policy.RequireRemarkOnVariance, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Task{}, err
	}
	t.Status = domain.Status(status)
	rows, err := s.db(ctx).Query(ctx, `SELECT line_id,item_id,item_name,barcode,expected_quantity,received_quantity FROM receiving_line WHERE task_id=$1 ORDER BY line_id`, id)
	if err != nil {
		return domain.Task{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var line domain.Line
		if err := rows.Scan(&line.ID, &line.ItemID, &line.ItemName, &line.Barcode, &line.ExpectedQuantity, &line.ReceivedQuantity); err != nil {
			return domain.Task{}, err
		}
		t.Lines = append(t.Lines, line)
	}
	return t, rows.Err()
}
func (s *Store) Save(ctx context.Context, t domain.Task) error {
	tag, err := s.db(ctx).Exec(ctx, `UPDATE receiving_task SET status=$2,operator_id=$3,version=$4,updated_at=$5 WHERE task_id=$1`, t.ID, t.Status, t.OperatorID, t.Version, t.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	for _, line := range t.Lines {
		if _, err := s.db(ctx).Exec(ctx, `UPDATE receiving_line SET received_quantity=$2 WHERE line_id=$1`, line.ID, line.ReceivedQuantity); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) List(ctx context.Context, f ports.Filter) (ports.Page, error) {
	after := ""
	if f.Cursor != "" {
		v, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil {
			return ports.Page{}, fmt.Errorf("invalid cursor")
		}
		after = string(v)
	}
	rows, err := s.db(ctx).Query(ctx, `SELECT task_id FROM receiving_task WHERE warehouse_id=$1 AND (operator_id IS NULL OR operator_id=$2) AND ($3='' OR status=$3) AND task_id>$4 ORDER BY task_id LIMIT $5`, f.WarehouseID, f.OperatorID, f.Status, after, f.Limit+1)
	if err != nil {
		return ports.Page{}, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return ports.Page{}, err
		}
		ids = append(ids, id)
	}
	page := ports.Page{}
	if len(ids) > f.Limit {
		cursor := base64.RawURLEncoding.EncodeToString([]byte(ids[f.Limit-1]))
		page.NextCursor = &cursor
		ids = ids[:f.Limit]
	}
	for _, id := range ids {
		task, err := s.Get(ctx, id)
		if err != nil {
			return ports.Page{}, err
		}
		page.Items = append(page.Items, task)
	}
	return page, nil
}
func (s *Store) ResolveBarcode(ctx context.Context, taskID, barcode string) (domain.Line, error) {
	var line domain.Line
	err := s.db(ctx).QueryRow(ctx, `SELECT line_id,item_id,item_name,barcode,expected_quantity,received_quantity FROM receiving_line WHERE task_id=$1 AND barcode=$2`, taskID, barcode).Scan(&line.ID, &line.ItemID, &line.ItemName, &line.Barcode, &line.ExpectedQuantity, &line.ReceivedQuantity)
	if err == nil {
		return line, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Line{}, err
	}
	var exists bool
	if err := s.db(ctx).QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM receiving_line WHERE barcode=$1)`, barcode).Scan(&exists); err != nil {
		return domain.Line{}, err
	}
	if exists {
		return domain.Line{}, domain.ErrBarcodeWrongContext
	}
	return domain.Line{}, domain.ErrBarcodeUnknown
}
func (s *Store) ApplyInventory(ctx context.Context, warehouse, item string, delta int64) error {
	_, err := s.db(ctx).Exec(ctx, `INSERT INTO inventory_balance(warehouse_id,item_id,quantity,version,updated_at) VALUES($1,$2,$3,1,now()) ON CONFLICT(warehouse_id,item_id) DO UPDATE SET quantity=inventory_balance.quantity+EXCLUDED.quantity,version=inventory_balance.version+1,updated_at=now()`, warehouse, item, delta)
	return err
}
func (s *Store) FindByKey(ctx context.Context, key string) (ports.CommandStatus, bool, error) {
	if _, err := s.db(ctx).Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return ports.CommandStatus{}, false, err
	}
	return s.find(ctx, `idempotency_key`, key)
}
func (s *Store) FindByID(ctx context.Context, id uuid.UUID) (ports.CommandStatus, bool, error) {
	return s.find(ctx, `command_id`, id)
}
func (s *Store) find(ctx context.Context, column string, value any) (ports.CommandStatus, bool, error) {
	var v ports.CommandStatus
	err := s.db(ctx).QueryRow(ctx, `SELECT command_id,command_type,status,warehouse_id,operator_id,result_json FROM receiving_command_status WHERE `+column+`=$1`, value).Scan(&v.CommandID, &v.CommandType, &v.Status, &v.WarehouseID, &v.OperatorID, &v.Result)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.CommandStatus{}, false, nil
	}
	return v, err == nil, err
}
func (s *Store) SaveCommand(ctx context.Context, key string, v ports.CommandStatus) error {
	_, err := s.db(ctx).Exec(ctx, `INSERT INTO receiving_command_status(command_id,idempotency_key,command_type,status,warehouse_id,operator_id,result_json) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.CommandID, key, v.CommandType, v.Status, v.WarehouseID, v.OperatorID, v.Result)
	return err
}

type Commands struct{ Store *Store }

func (c Commands) FindByKey(ctx context.Context, key string) (ports.CommandStatus, bool, error) {
	return c.Store.FindByKey(ctx, key)
}
func (c Commands) FindByID(ctx context.Context, id uuid.UUID) (ports.CommandStatus, bool, error) {
	return c.Store.FindByID(ctx, id)
}
func (c Commands) Save(ctx context.Context, key string, v ports.CommandStatus) error {
	return c.Store.SaveCommand(ctx, key, v)
}
func (s *Store) Append(ctx context.Context, e event.DomainEventEnvelope) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.db(ctx).Exec(ctx, `INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json) VALUES($1,$2,$3,$4,$5)`, e.EventID, e.AggregateID, e.AggregateVersion, e.EventType, data)
	return err
}
func (s *Store) AppendAudit(ctx context.Context, action, outcome string, t domain.Task, actor platform.ActorContext, details json.RawMessage) error {
	_, err := s.db(ctx).Exec(ctx, `INSERT INTO audit_record(audit_id,action,outcome,aggregate_id,operator_id,device_id,warehouse_id,correlation_id,occurred_at,details_json) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now(),$9)`, uuid.New(), action, outcome, t.ID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID, details)
	return err
}
func (s *Store) AppendDenied(ctx context.Context, action, aggregateID string, actor platform.ActorContext, code string) error {
	details, _ := json.Marshal(map[string]string{"errorCode": code})
	_, err := s.db(ctx).Exec(ctx, `INSERT INTO audit_record(audit_id,action,outcome,aggregate_id,operator_id,device_id,warehouse_id,correlation_id,occurred_at,details_json) VALUES($1,$2,'DENIED',$3,$4,$5,$6,$7,now(),$8)`, uuid.New(), action, aggregateID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID, details)
	return err
}
func (s *Store) InvalidateReceivingViews(context.Context, string, string) error { return nil }
