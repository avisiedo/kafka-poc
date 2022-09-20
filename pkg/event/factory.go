package event

import (
	"context"
	"my-test-app/pkg/config"
	"my-test-app/pkg/event/schema"
	"my-test-app/pkg/utils"
	"sync"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func Start(
	ctx context.Context,
	cfg *config.Configuration,
	db *gorm.DB,
	errors chan<- error,
	// ready *utils.ProbeHandler,
	// live *utils.ProbeHandler,
	wg *sync.WaitGroup,
	handler Eventable,
) {
	// instrumentation.Start()

	// schemaMapper := make(map[string]*jsonschema.Schema)
	// var schemaNames = []string{"schema.message.response", "schema.satmessage.response"}
	var (
		schemas schema.TopicSchemas
		err     error
	)
	schemas, err = schema.LoadSchemas()
	utils.DieOnError(err)

	// schemas := utils.LoadSchemas(cfg, schemaNames)
	// schemaMapper[runnerMessageHeaderValue] = schemas[0]
	// schemaMapper[satMessageHeaderValue] = schemas[1]

	// TODO Connect to the database
	// db, sql := db.Connect(ctx, cfg)
	// ready.Register(sql.Ping)
	// live.Register(sql.Ping)

	// kafkaTimeout := cfg.GetInt("kafka.timeout")
	topics := cfg.Kafka.Topics
	consumer, err := NewConsumer(ctx, cfg, topics)
	utils.DieOnError(err)

	// ready.Register(func() error {
	// 	return kafka.Ping(kafkaTimeout, consumer)
	// })

	// headerPredicate := FilterByHeaderPredicate(ctx, requestTypeHeader, runnerMessageHeaderValue, satMessageHeaderValue)
	// validationPredicate := SchemaValidationPredicate(ctx, requestTypeHeader, schemaMapper)

	// TODO Add here the schema validation

	// start := NewConsumerEventLoop(ctx, consumer, headerPredicate, validationPredicate, handler.onMessage, errors)
	start := NewConsumerEventLoop(ctx, consumer /* nil, nil, */, schemas, handler.OnMessage)

	go func() {
		defer wg.Done()
		defer log.Logger.Debug().Msg("Response consumer stopped")
		// TODO Disconnect from the database
		// defer db.Close()
		defer consumer.Close()
		wg.Add(1)
		start()
	}()
}
