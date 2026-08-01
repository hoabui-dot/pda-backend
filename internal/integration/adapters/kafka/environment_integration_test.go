package kafka

import "testing"

func TestSharedMESKafkaEnvironmentProvidesRequiredPDA8Topics(t *testing.T) {
	ensureTestTopics(t,
		"pda.task.events",
		"pda.receiving.events",
		"pda.movement.events",
		"pda.inventory.events",
		"pda.shipping.events",
		"pda.dlq",
	)
}
