package fixture

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

type Versioned struct {
	FixtureVersion int `json:"fixtureVersion"`
}

type Loader struct{ filesystem fs.FS }

func NewLoader(filesystem fs.FS) Loader { return Loader{filesystem: filesystem} }

func Load[T any](loader Loader, path string) (T, error) {
	var result T
	data, err := fs.ReadFile(loader.filesystem, path)
	if err != nil {
		return result, fmt.Errorf("read fixture %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("decode fixture %s: %w", path, err)
	}
	var versioned Versioned
	if err := json.Unmarshal(data, &versioned); err != nil || versioned.FixtureVersion < 1 {
		return result, fmt.Errorf("fixture %s requires a positive fixtureVersion", path)
	}
	return result, nil
}
