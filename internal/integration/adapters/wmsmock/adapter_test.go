package wmsmock

import (
	"context"
	"os"
	"testing"
)

func TestAdapterLoadsDeterministicWarehouseFixture(t *testing.T) {
	adapter := New(os.DirFS("testdata"), "warehouses.json")
	warehouses, err := adapter.Warehouses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(warehouses) != 1 || warehouses[0].ID != "WH-01" {
		t.Fatalf("unexpected warehouses: %+v", warehouses)
	}
}
