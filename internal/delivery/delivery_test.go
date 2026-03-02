package delivery

import (
	"testing"
	"time"
)

func TestDeliveryLifecycleBasic(t *testing.T) {
	reg := NewDeliveryRegistry()
	id := reg.Create("test-delivery", map[string]interface{}{"a": 1})
	d, ok := reg.Get(id)
	if !ok {
		t.Fatalf("delivery not found after create")
	}
	if d.Status != DeliveryStatusPending {
		t.Fatalf("expected pending, got %v", d.Status)
	}
	reg.Update(id, DeliveryStatusInProgress)
	if d, _ := reg.Get(id); d.Status != DeliveryStatusInProgress {
		t.Fatalf("status not updated to in_progress, got %v", d.Status)
	}
	reg.Update(id, DeliveryStatusDone)
	if d, _ := reg.Get(id); d.Status != DeliveryStatusDone {
		t.Fatalf("status not updated to done, got %v", d.Status)
	}
	_ = time.Now()
}
