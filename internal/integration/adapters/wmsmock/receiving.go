package wmsmock

import (
	"context"
	"embed"

	"github.com/company/pda-backend/internal/execution/receiving/domain"
	"github.com/company/pda-backend/internal/platform/fixture"
)

//go:embed testdata/receiving-tasks.json
var receivingFixtures embed.FS

type receivingFixture struct {
	FixtureVersion int           `json:"fixtureVersion"`
	Tasks          []domain.Task `json:"tasks"`
}
type ReceivingAdapter struct{ loader fixture.Loader }

func NewReceivingAdapter() *ReceivingAdapter {
	return &ReceivingAdapter{fixture.NewLoader(receivingFixtures)}
}
func (a *ReceivingAdapter) Tasks(context.Context) ([]domain.Task, error) {
	v, err := fixture.Load[receivingFixture](a.loader, "testdata/receiving-tasks.json")
	if err != nil {
		return nil, err
	}
	return append([]domain.Task(nil), v.Tasks...), nil
}
