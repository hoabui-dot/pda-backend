package domain

type Operator struct {
	ID           string   `json:"id"`
	EmployeeCode string   `json:"employeeCode"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"displayName"`
	Password     string   `json:"-"`
	PasswordHash string   `json:"-"`
	Roles        []string `json:"roles"`
	Permissions  []string `json:"permissions"`
	WarehouseIDs []string `json:"warehouseIds"`
	ShiftCode    string   `json:"shiftCode"`
	Active       bool     `json:"active"`
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
