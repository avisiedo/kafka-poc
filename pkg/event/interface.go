package event

import (
	"context"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Eventable interface {
	OnMessage(ctx context.Context, msg *kafka.Message) error
}
