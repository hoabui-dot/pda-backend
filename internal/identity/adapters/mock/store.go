package mock

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	identity "github.com/company/pda-backend/internal/identity/domain"
	"github.com/company/pda-backend/internal/identity/ports"
)

//go:embed testdata/identity.json
var defaultFixtures embed.FS

type fixture struct {
	FixtureVersion int                  `json:"fixtureVersion"`
	Operators      []operatorFixture    `json:"operators"`
	Warehouses     []identity.Warehouse `json:"warehouses"`
}

type operatorFixture struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"displayName"`
	Password     string   `json:"password"`
	Roles        []string `json:"roles"`
	WarehouseIDs []string `json:"warehouseIds"`
}

type Store struct {
	mu         sync.RWMutex
	operators  map[string]identity.Operator
	byID       map[string]identity.Operator
	warehouses map[string]identity.Warehouse
	devices    map[string]identity.DeviceRegistration
	audit      []ports.AuditRecord
}

func Load(filesystem fs.FS, path string) (*Store, error) {
	data, err := fs.ReadFile(filesystem, path)
	if err != nil {
		return nil, err
	}
	var loaded fixture
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	if loaded.FixtureVersion < 1 {
		return nil, fmt.Errorf("fixtureVersion must be positive")
	}
	store := &Store{operators: map[string]identity.Operator{}, byID: map[string]identity.Operator{}, warehouses: map[string]identity.Warehouse{}, devices: map[string]identity.DeviceRegistration{}}
	for _, record := range loaded.Operators {
		operator := identity.Operator{ID: record.ID, Username: record.Username, DisplayName: record.DisplayName, Password: record.Password, Roles: record.Roles, WarehouseIDs: record.WarehouseIDs}
		store.operators[operator.Username] = operator
		store.byID[operator.ID] = operator
	}
	for _, warehouse := range loaded.Warehouses {
		store.warehouses[warehouse.ID] = warehouse
	}
	return store, nil
}

func LoadDefault() (*Store, error) { return Load(defaultFixtures, "testdata/identity.json") }

func (s *Store) ByUsername(_ context.Context, username string) (identity.Operator, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.operators[username]
	return value, ok, nil
}
func (s *Store) ByID(_ context.Context, id string) (identity.Operator, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.byID[id]
	return value, ok, nil
}
func (s *Store) Warehouses(_ context.Context, ids []string) ([]identity.Warehouse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]identity.Warehouse, 0, len(ids))
	for _, id := range ids {
		if value, ok := s.warehouses[id]; ok {
			result = append(result, value)
		}
	}
	return result, nil
}
func (s *Store) Register(_ context.Context, registration identity.DeviceRegistration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.devices[registration.DeviceID] = registration
	return nil
}
func (s *Store) IsRegistered(_ context.Context, registration identity.DeviceRegistration) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	actual, ok := s.devices[registration.DeviceID]
	return ok && actual == registration, nil
}
func (s *Store) Append(_ context.Context, record ports.AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, record)
	return nil
}
func (s *Store) AuditRecords() []ports.AuditRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ports.AuditRecord(nil), s.audit...)
}
