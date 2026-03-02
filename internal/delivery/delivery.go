package delivery

import "time"

type Delivery struct {
	ID        string
	Name      string
	Payload   map[string]interface{}
	Status    DeliveryStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}
