# PDA WMS Backend — Go Code Patterns and Reference Skeletons

## 1. Explicit domain types and commands

```go
type ReceivingTaskID string
type ReceivingLineID string
type Barcode string

type ConfirmReceiptCommand struct {
	CommandID     uuid.UUID
	IdempotencyKey string
	TaskID        ReceivingTaskID
	LineID        ReceivingLineID
	Barcode       Barcode
	Quantity      int64
	Remark        *string
	BaseVersion   int64
	Actor         ActorContext
}
```

Validate domain values in constructors. Do not use pointer fields merely to avoid validation, and never expose persistence structs as domain models.

## 2. Actor and command metadata

```go
type ActorContext struct {
	OperatorID, DeviceID, WarehouseID, CorrelationID string
}

type CommandMetadata struct {
	CommandID uuid.UUID
	IdempotencyKey string
	IssuedAt time.Time
	Actor ActorContext
}
```

## 3. Typed results and errors

```go
type DomainError struct {
	Code string
	SafeMessage string
	Details map[string]string
	Retryable bool
}

func (e *DomainError) Error() string { return e.Code }
```

Return `(T, error)` and use `errors.Is`/`errors.As`. Domain error codes are stable API values; raw dependency errors never cross the transport boundary.

## 4. Application transaction boundary

```go
func (s *ReceivingService) Confirm(ctx context.Context, cmd ConfirmReceiptCommand) (ReceiptResult, error) {
	return s.tx.WithinTransaction(ctx, func(txCtx context.Context) (ReceiptResult, error) {
		if previous, ok, err := s.idempotency.Find(txCtx, cmd.IdempotencyKey); err != nil {
			return ReceiptResult{}, err
		} else if ok {
			return previous.ReceiptResult()
		}

		task, err := s.receiving.GetForUpdate(txCtx, cmd.TaskID)
		if err != nil { return ReceiptResult{}, err }
		if err := s.authorization.CheckReceiving(cmd.Actor, task); err != nil { return ReceiptResult{}, err }
		event, err := task.Confirm(cmd.LineID, cmd.Barcode, cmd.Quantity, cmd.Remark, cmd.BaseVersion)
		if err != nil { return ReceiptResult{}, err }

		if err := s.receiving.Save(txCtx, task); err != nil { return ReceiptResult{}, err }
		if err := s.idempotency.SaveSuccess(txCtx, cmd.IdempotencyKey, task.Version()); err != nil { return ReceiptResult{}, err }
		if err := s.outbox.Append(txCtx, event.Envelope(cmd.Actor, cmd.CommandID)); err != nil { return ReceiptResult{}, err }
		return task.ReceiptResult(), nil
	})
}
```

## 5. Event publisher port and mock adapter

```go
type DomainEventPublisher interface {
	Publish(context.Context, DomainEventEnvelope) error
}

type MockDomainEventPublisher struct { log MockEventLogRepository }

func (p *MockDomainEventPublisher) Publish(ctx context.Context, event DomainEventEnvelope) error {
	return p.log.Append(ctx, event)
}
```

Composition code selects exactly one adapter from validated configuration. Business packages never import Kafka client types.

## 6. Outbox and inbox

Outbox publishers claim committed rows with `FOR UPDATE SKIP LOCKED`, publish using the aggregate ID as stable key, record bounded retries, and mark success. Consumers insert the event ID into an inbox in the same transaction as their side effect; a duplicate insert is a successful no-op.

## 7. Cache aside

```go
func (q *DashboardQuery) Get(ctx context.Context, key CacheKey) (DashboardView, error) {
	if value, ok := q.cache.Get(ctx, key); ok { return value, nil }
	value, err := q.projection.Load(ctx, key)
	if err != nil { return DashboardView{}, err }
	_ = q.cache.Put(ctx, key, value, q.ttl) // cache failure does not rewrite DB truth
	return value, nil
}
```

Commit mutations before cache invalidation. Redis never authorizes or becomes authoritative for a warehouse mutation.

## 8. HTTP transport

Handlers decode and validate transport DTOs, call one application port, and encode the standard envelope. They do not open database transactions or access repositories. Middleware supplies correlation, authenticated operator, device, and warehouse context.

## 9. Package rules

```text
internal/execution/receiving/
  domain/
  application/
  ports/
  adapters/http/
  adapters/postgres/
  adapters/messaging/
```

Go `internal` boundaries prevent cross-service imports. Avoid generic `utils`, `helpers`, or shared mutable domain packages.

## 10. Architecture tests

Use `go list -deps -json`, AST/import inspection, and focused tests to enforce:

- domain packages import only the standard library and approved domain packages;
- HTTP adapters do not import PostgreSQL adapters;
- Kafka and Redis clients exist only in infrastructure adapters;
- transport DTOs and persistence records do not enter domain APIs;
- one bounded context cannot import another context's persistence package;
- Android, Java, Kotlin, Spring, and Gradle artifacts cannot enter the backend repository.
