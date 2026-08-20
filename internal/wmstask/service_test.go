package wmstask

import "testing"

func TestDeliveryEvidenceEventIDIsStablePerLogicalAck(t *testing.T) {
	actor := Actor{OperatorID: "operator-1", DeviceID: "device-1"}
	first := deliveryEvidenceEventID("delivery-1", actor)
	second := deliveryEvidenceEventID("delivery-1", actor)
	if first != second {
		t.Fatal("same delivery/operator/device must produce one evidence event ID")
	}
	if first == deliveryEvidenceEventID("delivery-2", actor) {
		t.Fatal("different delivery events must not share evidence event IDs")
	}
	if first == deliveryEvidenceEventID("delivery-1", Actor{OperatorID: "operator-2", DeviceID: "device-1"}) {
		t.Fatal("different operators must not share evidence event IDs")
	}
}
