package wmsmock

import (
	"context"
	"embed"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/platform/fixture"
)

//go:embed testdata/tasks.json
var taskFixtures embed.FS

type taskFixture struct {
	FixtureVersion int           `json:"fixtureVersion"`
	Tasks          []domain.Task `json:"tasks"`
}
type TaskAdapter struct{ loader fixture.Loader }

func NewTaskAdapter() *TaskAdapter { return &TaskAdapter{loader: fixture.NewLoader(taskFixtures)} }
func (a *TaskAdapter) Tasks(_ context.Context) ([]domain.Task, error) {
	loaded, err := fixture.Load[taskFixture](a.loader, "testdata/tasks.json")
	if err != nil {
		return nil, err
	}
	return append([]domain.Task(nil), loaded.Tasks...), nil
}
