package kafka

import "testing"

func TestSharedMESKafkaEnvironmentProvidesRequiredPDA8Topics(t *testing.T) {
	ensureTestTopics(t,
		"pda.task.events.v1",
		"pda.receiving.events.v1",
		"pda.movement.events.v1",
		"pda.inventory.events.v1",
		"pda.shipping.events.v1",
		"pda.dlq",
	)
}
