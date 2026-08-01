package wmsmock

import (
	"context"
	"io/fs"

	"github.com/company/pda-backend/internal/integration/ports"
	"github.com/company/pda-backend/internal/platform/fixture"
)

type warehouseFixture struct {
	FixtureVersion int               `json:"fixtureVersion"`
	Warehouses     []ports.Warehouse `json:"warehouses"`
}

type Adapter struct {
	loader fixture.Loader
	path   string
}

func New(filesystem fs.FS, path string) *Adapter {
	return &Adapter{loader: fixture.NewLoader(filesystem), path: path}
}

func (a *Adapter) Warehouses(_ context.Context) ([]ports.Warehouse, error) {
	loaded, err := fixture.Load[warehouseFixture](a.loader, a.path)
	if err != nil {
		return nil, err
	}
	return append([]ports.Warehouse(nil), loaded.Warehouses...), nil
}
