package event

import (
	"context"
	"my-test-app/pkg/event/schema"
	"sync"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type handler struct {
	db *gorm.DB
}

func (h *handler) onMessage(ctx context.Context, msg *kafka.Message) {
	log.Debug().Msg("onMessage was called")
	return
}

func Start(
	ctx context.Context,
	cfg *viper.Viper,
	errors chan<- error,
	db *gorm.DB,
	// ready *utils.ProbeHandler,
	// live *utils.ProbeHandler,
	wg *sync.WaitGroup,
) {
	// instrumentation.Start()

	// schemaMapper := make(map[string]*jsonschema.Schema)
	// var schemaNames = []string{"schema.message.response", "schema.satmessage.response"}
	var (
		schemas schema.TopicSchemas
		err     error
	)
	schemas, err = schema.LoadSchemas()
	DieOnError(err)

	// TODO Prepare here how to inject the schema validation
	schemas = schemas

	// schemas := utils.LoadSchemas(cfg, schemaNames)
	// schemaMapper[runnerMessageHeaderValue] = schemas[0]
	// schemaMapper[satMessageHeaderValue] = schemas[1]

	// TODO Connect to the database
	// db, sql := db.Connect(ctx, cfg)
	// ready.Register(sql.Ping)
	// live.Register(sql.Ping)

	// kafkaTimeout := cfg.GetInt("kafka.timeout")
	topics := cfg.GetStringSlice("kafka.topics")
	consumer, err := NewConsumer(ctx, cfg, topics)
	DieOnError(err)

	// ready.Register(func() error {
	// 	return kafka.Ping(kafkaTimeout, consumer)
	// })

	handler := &handler{
		db: db,
	}

	// headerPredicate := FilterByHeaderPredicate(ctx, requestTypeHeader, runnerMessageHeaderValue, satMessageHeaderValue)
	// validationPredicate := SchemaValidationPredicate(ctx, requestTypeHeader, schemaMapper)

	// TODO Add here the schema validation

	// start := NewConsumerEventLoop(ctx, consumer, headerPredicate, validationPredicate, handler.onMessage, errors)
	start := NewConsumerEventLoop(ctx, consumer, nil, nil, handler.onMessage, errors)

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
