package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/company/pda-backend/internal/inventory/domain"
	"github.com/company/pda-backend/internal/inventory/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
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
	if x, ok := c.Value(txKey{}).(pgx.Tx); ok {
		return x
	}
	return s.pool
}
func (s *Store) WithinTransaction(c context.Context, f func(context.Context) error) error {
	x, e := s.pool.Begin(c)
	if e != nil {
		return e
	}
	defer x.Rollback(c)
	if e = f(context.WithValue(c, txKey{}, x)); e != nil {
		return e
	}
	return x.Commit(c)
}
func (s *Store) Search(c context.Context, w, q string) ([]domain.Balance, error) {
	return s.Balances(c, w, q, q)
}
func (s *Store) Balances(c context.Context, w, item, loc string) ([]domain.Balance, error) {
	rows, e := s.d(c).Query(c, "SELECT warehouse_id,location_id,item_id,quantity,version,updated_at FROM inventory_location_balance WHERE warehouse_id=$1 AND($2='' OR item_id ILIKE '%'||$2||'%')AND($3='' OR location_id ILIKE '%'||$3||'%') ORDER BY location_id,item_id", w, item, loc)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Balance
	for rows.Next() {
		var x domain.Balance
		if e = rows.Scan(&x.WarehouseID, &x.LocationID, &x.ItemID, &x.Quantity, &x.Version, &x.AsOf); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) Movements(c context.Context, w, item, cursor string) ([]domain.Movement, error) {
	rows, e := s.d(c).Query(c, "SELECT movement_id,task_id,workflow,warehouse_id,item_id,source_location,destination_location,quantity,aggregate_version,occurred_at FROM inventory_movement WHERE warehouse_id=$1 AND($2='' OR item_id=$2)AND($3='' OR movement_id::text<$3) ORDER BY occurred_at DESC,movement_id DESC LIMIT 50", w, item, cursor)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.Movement
	for rows.Next() {
		var x domain.Movement
		if e = rows.Scan(&x.ID, &x.TaskID, &x.Workflow, &x.WarehouseID, &x.ItemID, &x.SourceLocation, &x.DestinationLocation, &x.Quantity, &x.AggregateVersion, &x.OccurredAt); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) ValidateLocations(c context.Context, w, src, dst, item string, q int64) error {
	if src == dst {
		return domain.ErrSameLocation
	}
	var stock int64
	e := s.d(c).QueryRow(c, "SELECT quantity FROM inventory_location_balance WHERE warehouse_id=$1 AND location_id=$2 AND item_id=$3 FOR UPDATE", w, src, item).Scan(&stock)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.ErrSource
	}
	if e != nil {
		return e
	}
	if stock < q {
		return domain.ErrStock
	}
	var exists bool
	e = s.d(c).QueryRow(c, "SELECT EXISTS(SELECT 1 FROM warehouse_location WHERE warehouse_id=$1 AND location_id=$2)", w, dst).Scan(&exists)
	if e != nil {
		return e
	}
	if !exists {
		return domain.ErrDestination
	}
	return nil
}
func (s *Store) Transfer(c context.Context, t domain.Transfer) error {
	q := t.Quantity
	tag, e := s.d(c).Exec(c, "UPDATE inventory_location_balance SET quantity=quantity-$4,version=version+1,updated_at=$5 WHERE warehouse_id=$1 AND location_id=$2 AND item_id=$3 AND quantity >= $4", t.WarehouseID, t.SourceLocation, t.ItemID, q, t.AsOf)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return domain.ErrStock
	}
	_, e = s.d(c).Exec(c, "INSERT INTO inventory_location_balance VALUES($1,$2,$3,$4,1,$5)ON CONFLICT(warehouse_id,location_id,item_id)DO UPDATE SET quantity=inventory_location_balance.quantity+$4,version=inventory_location_balance.version+1,updated_at=$5", t.WarehouseID, t.DestinationLocation, t.ItemID, q, t.AsOf)
	if e != nil {
		return e
	}
	id, _ := uuid.Parse(t.CommandID)
	_, e = s.d(c).Exec(c, "INSERT INTO inventory_movement VALUES($1,$2,'TRANSFER',$3,$4,$5,$6,$7,1,$8)", id, t.CommandID, t.WarehouseID, t.ItemID, t.SourceLocation, t.DestinationLocation, q, t.AsOf)
	return e
}
func (s *Store) Find(c context.Context, key string) (ports.CommandResult, bool, error) {
	if _, e := s.d(c).Exec(c, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); e != nil {
		return ports.CommandResult{}, false, e
	}
	var x ports.CommandResult
	e := s.d(c).QueryRow(c, "SELECT command_id,warehouse_id,operator_id,result_json FROM inventory_command_status WHERE idempotency_key=$1", key).Scan(&x.CommandID, &x.WarehouseID, &x.OperatorID, &x.Result)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, false, nil
	}
	return x, e == nil, e
}
func (s *Store) SaveCommand(c context.Context, key, typ string, x ports.CommandResult) error {
	_, e := s.d(c).Exec(c, "INSERT INTO inventory_command_status VALUES($1,$2,$3,$4,$5,$6,now())", x.CommandID, key, typ, x.WarehouseID, x.OperatorID, x.Result)
	return e
}

type Commands struct{ Store *Store }

func (x Commands) Find(c context.Context, k string) (ports.CommandResult, bool, error) {
	return x.Store.Find(c, k)
}
func (x Commands) Save(c context.Context, k, t string, r ports.CommandResult) error {
	return x.Store.SaveCommand(c, k, t, r)
}
func (s *Store) Append(c context.Context, e event.DomainEventEnvelope) error {
	b, x := json.Marshal(e)
	if x != nil {
		return x
	}
	_, x = s.d(c).Exec(c, "INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json)VALUES($1,$2,$3,$4,$5)", e.EventID, e.AggregateID, e.AggregateVersion, e.EventType, b)
	return x
}
func (s *Store) AppendInventoryAudit(c context.Context, a, id string, actor platform.ActorContext, d json.RawMessage) error {
	_, e := s.d(c).Exec(c, "INSERT INTO audit_record VALUES($1,$2,'SUCCESS',$3,$4,$5,$6,$7,now(),$8)", uuid.New(), a, id, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID, d)
	return e
}
func (s *Store) InvalidateInventory(context.Context, string, string, string) error { return nil }
func (s *Store) ListCounts(c context.Context, w, op string) ([]domain.CountTask, error) {
	rows, e := s.d(c).Query(c, "SELECT task_id FROM cycle_count_task WHERE warehouse_id=$1 AND(operator_id IS NULL OR operator_id=$2)ORDER BY task_id", w, op)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []domain.CountTask
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		x, e := s.GetCount(c, id)
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) GetCount(c context.Context, id string) (domain.CountTask, error) {
	return s.count(c, id, "")
}
func (s *Store) GetCountForUpdate(c context.Context, id string) (domain.CountTask, error) {
	return s.count(c, id, " FOR UPDATE")
}
func (s *Store) count(c context.Context, id, suffix string) (domain.CountTask, error) {
	var t domain.CountTask
	e := s.d(c).QueryRow(c, "SELECT task_id,warehouse_id,operator_id,location_id,status,version,updated_at FROM cycle_count_task WHERE task_id=$1"+suffix, id).Scan(&t.ID, &t.WarehouseID, &t.OperatorID, &t.LocationID, &t.Status, &t.Version, &t.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return t, domain.ErrNotFound
	}
	if e != nil {
		return t, e
	}
	rows, e := s.d(c).Query(c, "SELECT line_id,item_id,expected_quantity,counted_quantity,variance,recount_required FROM cycle_count_line WHERE task_id=$1 ORDER BY line_id", id)
	if e != nil {
		return t, e
	}
	defer rows.Close()
	for rows.Next() {
		var l domain.CountLine
		if e = rows.Scan(&l.ID, &l.ItemID, &l.ExpectedQuantity, &l.CountedQuantity, &l.Variance, &l.RecountRequired); e != nil {
			return t, e
		}
		t.Lines = append(t.Lines, l)
	}
	return t, rows.Err()
}
func (s *Store) SaveCount(c context.Context, t domain.CountTask) error {
	_, e := s.d(c).Exec(c, "UPDATE cycle_count_task SET operator_id=$2,status=$3,version=$4,updated_at=$5 WHERE task_id=$1", t.ID, t.OperatorID, t.Status, t.Version, t.UpdatedAt)
	if e != nil {
		return e
	}
	_, e = s.d(c).Exec(c, "UPDATE warehouse_task SET operator_id=$2,status=$3,version=$4,updated_at=$5 WHERE task_id=$1", t.ID, t.OperatorID, t.Status, t.Version, t.UpdatedAt)
	if e != nil {
		return e
	}
	for _, l := range t.Lines {
		if _, e = s.d(c).Exec(c, "UPDATE cycle_count_line SET counted_quantity=$2,variance=$3,recount_required=$4 WHERE line_id=$1", l.ID, l.CountedQuantity, l.Variance, l.RecountRequired); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Seed(c context.Context) error {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	return s.WithinTransaction(c, func(x context.Context) error {
		_, e := s.d(x).Exec(x, "INSERT INTO warehouse_task(task_id,category,status,priority,warehouse_id,version,updated_at)VALUES('CC-001','CYCLE_COUNT','NEW',50,'WH-01',1,$1)ON CONFLICT DO NOTHING", now)
		if e != nil {
			return e
		}
		_, e = s.d(x).Exec(x, "INSERT INTO cycle_count_task VALUES('CC-001','WH-01',NULL,'PICK-01','NEW',1,$1)ON CONFLICT DO NOTHING", now)
		if e != nil {
			return e
		}
		_, e = s.d(x).Exec(x, "INSERT INTO cycle_count_line VALUES('CC-LINE-01','CC-001','ITEM-001',16,NULL,NULL,false)ON CONFLICT DO NOTHING")
		return e
	})
}

var _ = strings.TrimSpace
