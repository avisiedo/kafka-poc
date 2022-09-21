package event

import (
	"context"
	"my-test-app/pkg/config"
	"my-test-app/pkg/event/schema"
	"my-test-app/pkg/utils"

	"gorm.io/gorm"
)

// Adapted from: https://github.com/RedHatInsights/playbook-dispatcher/blob/master/internal/response-consumer/main.go#L21
func Start(
	ctx context.Context,
	cfg *config.Configuration,
	db *gorm.DB,
	handler Eventable,
) {
	var (
		schemas schema.TopicSchemas
		err     error
	)
	schemas, err = schema.LoadSchemas()
	utils.DieOnError(err)

	topics := cfg.Kafka.Topics
	consumer, err := NewConsumer(ctx, cfg, topics)
	utils.DieOnError(err)

	start := NewConsumerEventLoop(ctx, consumer /* nil, nil, */, schemas, handler.OnMessage)
	start()
}
