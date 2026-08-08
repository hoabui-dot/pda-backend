package wmshttp

import (
	"context"
	executionapp "github.com/company/pda-backend/internal/execution/application"
	executiondomain "github.com/company/pda-backend/internal/execution/domain"
	executionports "github.com/company/pda-backend/internal/execution/ports"
	gatewayports "github.com/company/pda-backend/internal/gateway/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
)

// TaskAdapter maps Warehouse Execution task reads and claim/release commands
// to the PDA task use-case boundary. It does not persist a second task copy.
type TaskAdapter struct{ client *Client }

func NewTaskAdapter(client *Client) *TaskAdapter { return &TaskAdapter{client: client} }

func (a *TaskAdapter) List(ctx context.Context, filter executionports.TaskFilter, actor platform.ActorContext) (executionports.TaskPage, error) {
	rows, err := a.client.ListExecutionTasks(ctx, actor.WarehouseID, actor.OperatorID, filter.Category, filter.Status, filter.Query, filter.Limit)
	if err != nil {
		return executionports.TaskPage{}, err
	}
	items := make([]executiondomain.Task, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapExecutionTask(row))
	}
	return executionports.TaskPage{Tasks: items}, nil
}

func (a *TaskAdapter) Detail(ctx context.Context, id string, actor platform.ActorContext) (executiondomain.Task, error) {
	row, err := a.client.GetExecutionTask(ctx, id)
	if err != nil {
		return executiondomain.Task{}, err
	}
	if row.WarehouseID != actor.WarehouseID || row.AssignedOperatorID != nil && *row.AssignedOperatorID != actor.OperatorID {
		return executiondomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Task access denied"}
	}
	return mapExecutionTask(row), nil
}

func (a *TaskAdapter) Claim(ctx context.Context, command executionapp.TaskCommand) (executiondomain.Task, error) {
	return a.command(ctx, command, "CLAIM")
}

func (a *TaskAdapter) Release(ctx context.Context, command executionapp.TaskCommand) (executiondomain.Task, error) {
	return a.command(ctx, command, "RELEASE")
}

func (a *TaskAdapter) command(ctx context.Context, command executionapp.TaskCommand, kind string) (executiondomain.Task, error) {
	row, err := a.client.ApplyExecutionTaskCommand(ctx, command.TaskID, executionTaskCommand{CommandID: command.CommandID.String(), CommandType: kind, ExpectedVersion: command.BaseVersion, CorrelationID: command.Actor.CorrelationID}, command.Actor.OperatorID, command.IdempotencyKey)
	if err != nil {
		return executiondomain.Task{}, err
	}
	return mapExecutionTask(row), nil
}

func (a *TaskAdapter) Summary(context.Context, string, string, platform.ActorContext) ([]executionports.SummaryItem, error) {
	return nil, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS task summary contract is not mapped"}
}

func (a *TaskAdapter) Dashboard(context.Context, platform.ActorContext) (executionports.Dashboard, error) {
	return executionports.Dashboard{}, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS task dashboard contract is not mapped"}
}

func (a *TaskAdapter) CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (executionports.IdempotencyResult, error) {
	return executionports.IdempotencyResult{}, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS task command status contract is not mapped"}
}

func mapExecutionTask(row executionTask) executiondomain.Task {
	category := executiondomain.TaskCategory(row.TaskType)
	switch category {
	case executiondomain.CategoryPutaway, executiondomain.CategoryPicking, executiondomain.CategoryReplenishment, executiondomain.CategoryReceiving, executiondomain.CategoryCycleCount:
	default:
		category = executiondomain.TaskCategory("UNKNOWN")
	}
	status := executiondomain.TaskStatus(row.Status)
	switch status {
	case executiondomain.StatusNew, executiondomain.StatusAssigned, executiondomain.StatusInProgress, executiondomain.StatusCompleted, executiondomain.StatusOnHold:
	default:
		status = executiondomain.StatusNew
	}
	return executiondomain.Task{ID: row.TaskID, Category: category, Status: status, Priority: row.Priority, Title: row.TaskType, WarehouseID: row.WarehouseID, OperatorID: row.AssignedOperatorID, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

var _ gatewayports.TaskOperations = (*TaskAdapter)(nil)
