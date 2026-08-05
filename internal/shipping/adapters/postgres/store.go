package postgres

import (
	"context"
	"encoding/json"
	"errors"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/shipping/domain"
	"github.com/company/pda-backend/internal/shipping/ports"
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
func (s *Store) Get(c context.Context, id string) (domain.Shipment, error) { return s.get(c, id, "") }
func (s *Store) GetForUpdate(c context.Context, id string) (domain.Shipment, error) {
	return s.get(c, id, " FOR UPDATE")
}
func (s *Store) get(c context.Context, id, suffix string) (domain.Shipment, error) {
	var x domain.Shipment
	e := s.d(c).QueryRow(c, "SELECT shipment_id,order_id,warehouse_id,status,carrier,tracking_number,picking_complete,version,updated_at FROM shipment WHERE shipment_id=$1"+suffix, id).Scan(&x.ID, &x.OrderID, &x.WarehouseID, &x.Status, &x.Carrier, &x.TrackingNumber, &x.PickingComplete, &x.Version, &x.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, domain.ErrNotFound
	}
	if e != nil {
		return x, e
	}
	rows, e := s.d(c).Query(c, "SELECT package_id,status,weight_grams FROM shipment_package WHERE shipment_id=$1 ORDER BY package_id", id)
	if e != nil {
		return x, e
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Package
		if e = rows.Scan(&p.ID, &p.Status, &p.WeightGrams); e != nil {
			return x, e
		}
		x.Packages = append(x.Packages, p)
	}
	return x, rows.Err()
}
func (s *Store) Save(c context.Context, x domain.Shipment) error {
	_, e := s.d(c).Exec(c, "UPDATE shipment SET status=$2,carrier=$3,tracking_number=$4,version=$5,updated_at=$6 WHERE shipment_id=$1", x.ID, x.Status, x.Carrier, x.TrackingNumber, x.Version, x.UpdatedAt)
	return e
}
func (s *Store) VerifyPackage(c context.Context, shipmentID, packageID string) error {
	tag, e := s.d(c).Exec(c, "UPDATE shipment_package SET status='VERIFIED' WHERE shipment_id=$1 AND package_id=$2 AND status NOT IN ('COMPLETED','VERIFIED')", shipmentID, packageID)
	if e != nil {
		return e
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if e = s.d(c).QueryRow(c, "SELECT EXISTS(SELECT 1 FROM shipment_package WHERE shipment_id=$1 AND package_id=$2)", shipmentID, packageID).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return domain.ErrPackageNotFound
		}
	}
	return nil
}
func (s *Store) ProjectPickingComplete(c context.Context, id string) error {
	_, e := s.d(c).Exec(c, "UPDATE shipment SET picking_complete=true,version=version+1,updated_at=now() WHERE shipment_id=$1", id)
	return e
}
func (s *Store) FindKey(c context.Context, k string) (ports.CommandResult, bool, error) {
	if _, e := s.d(c).Exec(c, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", k); e != nil {
		return ports.CommandResult{}, false, e
	}
	return s.find(c, "idempotency_key", k)
}
func (s *Store) FindID(c context.Context, id uuid.UUID) (ports.CommandResult, bool, error) {
	return s.find(c, "command_id", id)
}
func (s *Store) find(c context.Context, col string, v any) (ports.CommandResult, bool, error) {
	var x ports.CommandResult
	e := s.d(c).QueryRow(c, "SELECT command_id,shipment_id,warehouse_id,operator_id,result_json FROM shipping_command_status WHERE "+col+"=$1", v).Scan(&x.ID, &x.ShipmentID, &x.WarehouseID, &x.OperatorID, &x.Result)
	if errors.Is(e, pgx.ErrNoRows) {
		return x, false, nil
	}
	return x, e == nil, e
}
func (s *Store) SaveCommand(c context.Context, k string, x ports.CommandResult) error {
	_, e := s.d(c).Exec(c, "INSERT INTO shipping_command_status VALUES($1,$2,$3,$4,$5,'COMPLETED',$6,now())", x.ID, k, x.ShipmentID, x.WarehouseID, x.OperatorID, x.Result)
	return e
}

type Commands struct{ Store *Store }

func (x Commands) FindKey(c context.Context, k string) (ports.CommandResult, bool, error) {
	return x.Store.FindKey(c, k)
}
func (x Commands) FindID(c context.Context, id uuid.UUID) (ports.CommandResult, bool, error) {
	return x.Store.FindID(c, id)
}
func (x Commands) Save(c context.Context, k string, r ports.CommandResult) error {
	return x.Store.SaveCommand(c, k, r)
}
func (s *Store) Append(c context.Context, e event.DomainEventEnvelope) error {
	b, x := json.Marshal(e)
	if x != nil {
		return x
	}
	_, x = s.d(c).Exec(c, "INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json)VALUES($1,$2,$3,$4,$5)", e.EventID, e.AggregateID, e.AggregateVersion, e.EventType, b)
	return x
}
func (s *Store) AppendShippingAudit(c context.Context, a string, x domain.Shipment, actor platform.ActorContext, d json.RawMessage) error {
	_, e := s.d(c).Exec(c, "INSERT INTO audit_record VALUES($1,$2,'SUCCESS',$3,$4,$5,$6,$7,now(),$8)", uuid.New(), a, x.ID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID, d)
	return e
}
func (s *Store) InvalidateShippingViews(context.Context, string, string) error { return nil }
func (s *Store) Seed(c context.Context) error {
	n := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if _, e := s.pool.Exec(c, "INSERT INTO shipment VALUES('SHIP-001','ORDER-001','WH-01','PENDING',NULL,NULL,false,1,$1)ON CONFLICT DO NOTHING", n); e != nil {
		return e
	}
	_, e := s.pool.Exec(c, "INSERT INTO shipment_package VALUES('PKG-001','SHIP-001','COMPLETED',1200)ON CONFLICT DO NOTHING")
	return e
}
