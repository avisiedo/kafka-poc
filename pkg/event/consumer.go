package event

import (
	"context"
	"fmt"
	"my-test-app/pkg/config"
	"my-test-app/pkg/event/schema"
	"time"

	b64 "encoding/base64"
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
)

// https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md

func NewConsumer(ctx context.Context, config *config.Configuration, topics []string) (*kafka.Consumer, error) {
	// FIXME Clean-up the commented code
	// kafkaConfigMap := &kafka.ConfigMap{
	// 	"bootstrap.servers":        config.GetString("kafka.bootstrap.servers"),
	// 	"group.id":                 config.GetString("kafka.group.id"),
	// 	"auto.offset.reset":        config.GetString("kafka.auto.offset.reset"),
	// 	"auto.commit.interval.ms":  config.GetInt("kafka.auto.commit.interval.ms"),
	// 	"go.logs.channel.enable":   true,
	// 	"allow.auto.create.topics": true,
	// }
	kafkaConfigMap := &kafka.ConfigMap{
		"bootstrap.servers":        config.Kafka.Bootstrap.Servers,
		"group.id":                 config.Kafka.Group.Id,
		"auto.offset.reset":        config.Kafka.Auto.Offset.Reset,
		"auto.commit.interval.ms":  config.Kafka.Auto.Commit.Interval.Ms,
		"go.logs.channel.enable":   true,
		"allow.auto.create.topics": true,
	}

	// FIXME Clean-up the commented code
	// if config.Get("kafka.sasl.username") != nil {
	// 	_ = kafkaConfigMap.SetKey("sasl.username", config.GetString("kafka.sasl.username"))
	// 	_ = kafkaConfigMap.SetKey("sasl.password", config.GetString("kafka.sasl.password"))
	// 	_ = kafkaConfigMap.SetKey("sasl.mechanism", config.GetString("kafka.sasl.mechanism"))
	// 	_ = kafkaConfigMap.SetKey("security.protocol", config.GetString("kafka.sasl.protocol"))
	// 	_ = kafkaConfigMap.SetKey("ssl.ca.location", config.GetString("kafka.capath"))
	// }
	if config.Kafka.Sasl.Username != "" {
		_ = kafkaConfigMap.SetKey("sasl.username", config.Kafka.Sasl.Username)
		_ = kafkaConfigMap.SetKey("sasl.password", config.Kafka.Sasl.Password)
		_ = kafkaConfigMap.SetKey("sasl.mechanism", config.Kafka.Sasl.Mechnism)
		_ = kafkaConfigMap.SetKey("security.protocol", config.Kafka.Sasl.Protocol)
		_ = kafkaConfigMap.SetKey("ssl.ca.location", config.Kafka.Capath)
	}

	consumer, err := kafka.NewConsumer(kafkaConfigMap)
	if err != nil {
		return nil, err
	}

	err = consumer.SubscribeTopics(topics, nil)

	if err != nil {
		return nil, err
	}

	go func() {
		// log := utils.GetLogFromContext(ctx).Named("kafka")

		for {
			entry, ok := <-consumer.Logs()

			if !ok {
				return
			}

			// log.Debug(entry)
			log.Logger.
				Debug().
				Str("entry.Tag", entry.Tag).
				Str("entry.Name", entry.Name).
				Str("entry.Message", entry.Message)
		}
	}()

	return consumer, nil
}

func getHeader(msg *kafka.Message, key string) (*kafka.Header, error) {
	if msg == nil {
		return nil, fmt.Errorf("[getHeader] msg is nil")
	}
	for _, header := range msg.Headers {
		if header.Key == key {
			return &header, nil
		}
	}
	return nil, fmt.Errorf("[getHeader] could not find '%s' in message header", key)
}

// TODO Refactor to make this check dynamic so it does not need
//      to be modified if more different messages are added
func isValidEvent(event string) bool {
	switch event {
	case "introspect":
		return true
	default:
		return false
	}
}

// TODO Convert in a method for TopicSchemas
func getSchemaMap(schemas schema.TopicSchemas, topic string) schema.SchemaMap {
	schemaMap := (map[string]schema.SchemaMap)(schemas)
	if val, ok := schemaMap[topic]; ok {
		return val
	}
	return nil
}

// TODO Convert in a method for schema.SchemaMap
func getSchema(schemaMap schema.SchemaMap, event string) *schema.Schema {
	object := (map[string](*schema.Schema))(schemaMap)
	if val, ok := object[event]; ok {
		return val
	}
	return nil
}

// TODO Convert in a method for schema.SchemaMap
func validateMessage(schemas schema.TopicSchemas, msg *kafka.Message) error {
	var (
		err        error
		event      *kafka.Header
		sm         schema.SchemaMap
		s          *schema.Schema
		b64coded   []byte
		b64decoded []byte
	)
	if msg == nil {
		return fmt.Errorf("msg cannot be nil")
	}
	if event, err = getHeader(msg, "event"); err != nil {
		return fmt.Errorf("header 'event' not found: %w", err)
	}
	if !isValidEvent(event.String()) {
		return fmt.Errorf("event = '%s' is not valid", event.String())
	}
	if msg.TopicPartition.Topic == nil {
		return fmt.Errorf("topic cannot be nil")
	}
	topic := *msg.TopicPartition.Topic
	if sm = getSchemaMap(schemas, topic); sm == nil {
		return fmt.Errorf("topic '%s' not found in schema mapping", event.String())
	}
	if s = getSchema(sm, event.String()); s == nil {
		return fmt.Errorf("schema '%s' '%s' not found in schema mapping", event.String())
	}

	if err = json.Unmarshal(msg.Value, &b64coded); err != nil {
		return fmt.Errorf("[readMessage] Payload unmarshall error: %w", err)
	}
	if b64decoded, err = b64.StdEncoding.DecodeString(string(b64coded)); err != nil {
		return fmt.Errorf("[readMessage] Decoding base64 message value: %w", err)
	}

	return s.ValidateBytes(b64decoded)
}

func NewConsumerEventLoop(
	ctx context.Context,
	consumer *kafka.Consumer,
	schemas schema.TopicSchemas,
	handler func(context.Context, *kafka.Message),
) (start func()) {
	return func() {
		for {
			msg, err := consumer.ReadMessage(1 * time.Second) // TODO: configurable

			select {
			case <-ctx.Done():
				return
			default:
			}

			if err != nil {
				if err.(kafka.Error).Code() != kafka.ErrTimedOut {
					log.Logger.Warn().Msgf("[NewConsumerEventLoop] Error reading message from kafka: %w", err)
				}
				continue
			}

			if err = validateMessage(schemas, msg); err != nil {
				log.Logger.Error().Msgf("[NewConsumerEventLoop] Invalid message received: %s", string(msg.Value))
				continue
			}

			// if messagePredicate != nil && !messagePredicate(msg) {
			// 	continue
			// }

			// if validationPredicate != nil && !validationPredicate(msg) {
			// 	continue
			// }

			// TODO Add here message schema validation

			handler(ctx, msg)
		}
	}
}
