package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/company/pda-backend/internal/execution/movement/domain"
	"github.com/company/pda-backend/internal/execution/movement/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type txKey struct{}
type db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Store struct{ pool *pgxpool.Pool }

func New(p *pgxpool.Pool) *Store { return &Store{p} }
func (s *Store) d(c context.Context) db {
	if t, ok := c.Value(txKey{}).(pgx.Tx); ok {
		return t
	}
	return s.pool
}
func (s *Store) WithinTransaction(c context.Context, f func(context.Context) error) error {
	t, e := s.pool.Begin(c)
	if e != nil {
		return e
	}
	defer t.Rollback(c)
	if e = f(context.WithValue(c, txKey{}, t)); e != nil {
		return e
	}
	return t.Commit(c)
}
func scan(row pgx.Row) (domain.Task, error) {
	var t domain.Task
	var w, st string
	e := row.Scan(&t.ID, &w, &st, &t.WarehouseID, &t.OperatorID, &t.Version, &t.SourceLocation, &t.DestinationLocation, &t.ItemID, &t.Barcode, &t.RequiredQuantity, &t.CompletedQuantity, &t.SourceValidated, &t.DestinationValidated, &t.ItemValidated, &t.UpdatedAt)
	t.Workflow = domain.Workflow(w)
	t.Status = domain.Status(st)
	if errors.Is(e, pgx.ErrNoRows) {
		return t, domain.ErrNotFound
	}
	return t, e
}

const fields = "task_id,workflow,status,warehouse_id,operator_id,version,source_location,destination_location,item_id,barcode,required_quantity,completed_quantity,source_validated,destination_validated,item_validated,updated_at"

func (s *Store) Get(c context.Context, id string) (domain.Task, error) {
	return scan(s.d(c).QueryRow(c, "SELECT "+fields+" FROM movement_task WHERE task_id=$1", id))
}
func (s *Store) GetForUpdate(c context.Context, id string) (domain.Task, error) {
	return scan(s.d(c).QueryRow(c, "SELECT "+fields+" FROM movement_task WHERE task_id=$1 FOR UPDATE", id))
}
func (s *Store) List(c context.Context, w domain.Workflow, warehouse, operator string) ([]domain.Task, error) {
	rows, e := s.d(c).Query(c, "SELECT "+fields+" FROM movement_task WHERE workflow=$1 AND warehouse_id=$2 AND(operator_id IS NULL OR operator_id=$3) ORDER BY task_id", w, warehouse, operator)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) Save(c context.Context, t domain.Task) error {
	tag, e := s.d(c).Exec(c, "UPDATE movement_task SET status=$2,operator_id=$3,version=$4,completed_quantity=$5,source_validated=$6,destination_validated=$7,item_validated=$8,updated_at=$9 WHERE task_id=$1", t.ID, t.Status, t.OperatorID, t.Version, t.CompletedQuantity, t.SourceValidated, t.DestinationValidated, t.ItemValidated, t.UpdatedAt)
	if e == nil && tag.RowsAffected() != 1 {
		return domain.ErrNotFound
	}
	if e != nil {
		return e
	}
	_, e = s.d(c).Exec(c, "UPDATE warehouse_task SET status=$2,operator_id=$3,version=$4,updated_at=$5 WHERE task_id=$1", t.ID, t.Status, t.OperatorID, t.Version, t.UpdatedAt)
	return e
}
func (s *Store) Suggestions(c context.Context, t domain.Task) ([]domain.Location, error) {
	rows, e := s.d(c).Query(c, "SELECT location_id,zone,capacity,used_capacity,compatible_item_id FROM warehouse_location WHERE warehouse_id=$1 AND capacity-used_capacity >= $2 AND(compatible_item_id IS NULL OR compatible_item_id=$3) ORDER BY location_id", t.WarehouseID, t.Remaining(), t.ItemID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Location
	for rows.Next() {
		var l domain.Location
		if e = rows.Scan(&l.ID, &l.Zone, &l.Capacity, &l.UsedCapacity, &l.CompatibleItemID); e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (s *Store) CheckMove(c context.Context, t domain.Task, q int64) error {
	var stock int64
	e := s.d(c).QueryRow(c, "SELECT quantity FROM inventory_location_balance WHERE warehouse_id=$1 AND location_id=$2 AND item_id=$3 FOR UPDATE", t.WarehouseID, t.SourceLocation, t.ItemID).Scan(&stock)
	if errors.Is(e, pgx.ErrNoRows) || stock < q {
		return domain.ErrStock
	}
	if e != nil {
		return e
	}
	var cap, used int64
	var compatible *string
	e = s.d(c).QueryRow(c, "SELECT capacity,used_capacity,compatible_item_id FROM warehouse_location WHERE warehouse_id=$1 AND location_id=$2 FOR UPDATE", t.WarehouseID, t.DestinationLocation).Scan(&cap, &used, &compatible)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ErrDestination
	}
	if e != nil {
		return e
	}
	if compatible != nil && *compatible != t.ItemID {
		return domain.ErrCapacity
	}
	if cap-used < q {
		return domain.ErrCapacity
	}
	return nil
}
func (s *Store) ApplyMove(c context.Context, t domain.Task, q int64) error {
	tag, e := s.d(c).Exec(c, "UPDATE inventory_location_balance SET quantity=quantity-$4,version=version+1,updated_at=now() WHERE warehouse_id=$1 AND location_id=$2 AND item_id=$3 AND quantity >= $4", t.WarehouseID, t.SourceLocation, t.ItemID, q)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrStock
	}
	_, e = s.d(c).Exec(c, "INSERT INTO inventory_location_balance(warehouse_id,location_id,item_id,quantity,version,updated_at)VALUES($1,$2,$3,$4,1,now())ON CONFLICT(warehouse_id,location_id,item_id)DO UPDATE SET quantity=inventory_location_balance.quantity+$4,version=inventory_location_balance.version+1,updated_at=now()", t.WarehouseID, t.DestinationLocation, t.ItemID, q)
	if e != nil {
		return e
	}
	if _, e = s.d(c).Exec(c, "UPDATE warehouse_location SET used_capacity=used_capacity-$3 WHERE warehouse_id=$1 AND location_id=$2", t.WarehouseID, t.SourceLocation, q); e != nil {
		return e
	}
	if _, e = s.d(c).Exec(c, "UPDATE warehouse_location SET used_capacity=used_capacity+$3 WHERE warehouse_id=$1 AND location_id=$2", t.WarehouseID, t.DestinationLocation, q); e != nil {
		return e
	}
	_, e = s.d(c).Exec(c, "INSERT INTO inventory_movement(movement_id,task_id,workflow,warehouse_id,item_id,source_location,destination_location,quantity,aggregate_version,occurred_at)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", uuid.New(), t.ID, t.Workflow, t.WarehouseID, t.ItemID, t.SourceLocation, t.DestinationLocation, q, t.Version, t.UpdatedAt)
	return e
}
func (s *Store) Find(c context.Context, key string) (ports.CommandResult, bool, error) {
	if _, e := s.d(c).Exec(c, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); e != nil {
		return ports.CommandResult{}, false, e
	}
	var r ports.CommandResult
	var w string
	e := s.d(c).QueryRow(c, "SELECT command_id,workflow,warehouse_id,operator_id,result_json FROM movement_command_status WHERE idempotency_key=$1", key).Scan(&r.CommandID, &w, &r.WarehouseID, &r.OperatorID, &r.Result)
	r.Workflow = domain.Workflow(w)
	if errors.Is(e, pgx.ErrNoRows) {
		return r, false, nil
	}
	return r, e == nil, e
}
func (s *Store) SaveCommand(c context.Context, key string, r ports.CommandResult) error {
	_, e := s.d(c).Exec(c, "INSERT INTO movement_command_status(command_id,idempotency_key,workflow,command_type,warehouse_id,operator_id,result_json)VALUES($1,$2,$3,'MUTATE',$4,$5,$6)", r.CommandID, key, r.Workflow, r.WarehouseID, r.OperatorID, r.Result)
	return e
}

type Commands struct{ Store *Store }

func (c Commands) Find(x context.Context, k string) (ports.CommandResult, bool, error) {
	return c.Store.Find(x, k)
}
func (c Commands) Save(x context.Context, k string, r ports.CommandResult) error {
	return c.Store.SaveCommand(x, k, r)
}
func (s *Store) Append(c context.Context, e event.DomainEventEnvelope) error {
	b, x := json.Marshal(e)
	if x != nil {
		return x
	}
	_, x = s.d(c).Exec(c, "INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json)VALUES($1,$2,$3,$4,$5)", e.EventID, e.AggregateID, e.AggregateVersion, e.EventType, b)
	return x
}
func (s *Store) AppendMovementAudit(c context.Context, a string, t domain.Task, actor platform.ActorContext, d json.RawMessage) error {
	_, e := s.d(c).Exec(c, "INSERT INTO audit_record(audit_id,action,outcome,aggregate_id,operator_id,device_id,warehouse_id,correlation_id,occurred_at,details_json)VALUES($1,$2,'SUCCESS',$3,$4,$5,$6,$7,now(),$8)", uuid.New(), a, t.ID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID, d)
	return e
}
func (s *Store) InvalidateMovementViews(context.Context, string, string) error { return nil }
func (s *Store) Seed(c context.Context, tasks []domain.Task) error {
	return s.WithinTransaction(c, func(x context.Context) error {
		for _, l := range []struct {
			id, zone  string
			cap, used int64
			item      *string
		}{{"STAGE-01", "STAGE", 100, 35, nil}, {"PICK-01", "PICK", 100, 20, str("ITEM-001")}, {"BULK-01", "BULK", 200, 80, nil}, {"PICK-02", "PICK", 25, 5, str("ITEM-002")}} {
			if _, e := s.d(x).Exec(x, "INSERT INTO warehouse_location VALUES('WH-01',$1,$2,$3,$4,$5)ON CONFLICT DO NOTHING", l.id, l.zone, l.cap, l.used, l.item); e != nil {
				return e
			}
		}
		for _, t := range tasks {
			if _, e := s.d(x).Exec(x, "INSERT INTO warehouse_task(task_id,category,status,priority,warehouse_id,operator_id,version,updated_at)VALUES($1,$2,$3,50,$4,$5,$6,$7)ON CONFLICT DO NOTHING", t.ID, t.Workflow, t.Status, t.WarehouseID, t.OperatorID, t.Version, t.UpdatedAt); e != nil {
				return e
			}
			if _, e := s.d(x).Exec(x, "INSERT INTO movement_task VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)ON CONFLICT DO NOTHING", t.ID, t.Workflow, t.Status, t.WarehouseID, t.OperatorID, t.Version, t.SourceLocation, t.DestinationLocation, t.ItemID, t.Barcode, t.RequiredQuantity, t.CompletedQuantity, t.SourceValidated, t.DestinationValidated, t.ItemValidated, t.UpdatedAt); e != nil {
				return e
			}
			if _, e := s.d(x).Exec(x, "INSERT INTO inventory_location_balance VALUES($1,$2,$3,$4,1,$5)ON CONFLICT DO NOTHING", t.WarehouseID, t.SourceLocation, t.ItemID, t.RequiredQuantity+10, time.Now().UTC()); e != nil {
				return e
			}
		}
		return nil
	})
}
func str(v string) *string { return &v }
