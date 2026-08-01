package ports

import "context"

type Warehouse struct{ ID, Name string }

type UpstreamWMS interface {
	Warehouses(context.Context) ([]Warehouse, error)
}
