package domain

type Operator struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"displayName"`
	Password     string   `json:"-"`
	Roles        []string `json:"roles"`
	WarehouseIDs []string `json:"warehouseIds"`
}

type Warehouse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DeviceRegistration struct {
	DeviceID    string `json:"deviceId"`
	OperatorID  string `json:"operatorId"`
	WarehouseID string `json:"warehouseId"`
}

func (o Operator) CanAccessWarehouse(id string) bool {
	for _, allowed := range o.WarehouseIDs {
		if allowed == id {
			return true
		}
	}
	return false
}
