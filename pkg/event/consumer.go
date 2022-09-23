package event

import (
	"fmt"
	"my-test-app/pkg/config"
	"my-test-app/pkg/event/message"
	"my-test-app/pkg/event/schema"
	"my-test-app/pkg/utils"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
)

// https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md
// https://docs.confluent.io/platform/current/clients/consumer.html#ak-consumer-configuration
func NewConsumer(config *config.Configuration) (*kafka.Consumer, error) {
	var (
		consumer *kafka.Consumer
		err      error
	)
	// FIXME Clean-up the commented code
	// kafkaConfigMap := &kafka.ConfigMap{
	// 	"bootstrap.servers":        config.GetString("kafka.bootstrap.servers"),
	// 	"group.id":                 config.GetString("kafka.group.id"),
	// 	"auto.offset.reset":        config.GetString("kafka.auto.offset.reset"),
	// 	"auto.commit.interval.ms":  config.GetInt("kafka.auto.commit.interval.ms"),
	// 	"go.logs.channel.enable":   true,
	// 	"allow.auto.create.topics": true,
	// }

	// if config.Get("kafka.sasl.username") != nil {
	// 	_ = kafkaConfigMap.SetKey("sasl.username", config.GetString("kafka.sasl.username"))
	// 	_ = kafkaConfigMap.SetKey("sasl.password", config.GetString("kafka.sasl.password"))
	// 	_ = kafkaConfigMap.SetKey("sasl.mechanism", config.GetString("kafka.sasl.mechanism"))
	// 	_ = kafkaConfigMap.SetKey("security.protocol", config.GetString("kafka.sasl.protocol"))
	// 	_ = kafkaConfigMap.SetKey("ssl.ca.location", config.GetString("kafka.capath"))
	// }

	kafkaConfigMap := &kafka.ConfigMap{
		"bootstrap.servers":        config.Kafka.Bootstrap.Servers,
		"group.id":                 config.Kafka.Group.Id,
		"auto.offset.reset":        config.Kafka.Auto.Offset.Reset,
		"auto.commit.interval.ms":  config.Kafka.Auto.Commit.Interval.Ms,
		"go.logs.channel.enable":   false,
		"allow.auto.create.topics": true,
	}

	if config.Kafka.Sasl.Username != "" {
		_ = kafkaConfigMap.SetKey("sasl.username", config.Kafka.Sasl.Username)
		_ = kafkaConfigMap.SetKey("sasl.password", config.Kafka.Sasl.Password)
		_ = kafkaConfigMap.SetKey("sasl.mechanism", config.Kafka.Sasl.Mechnism)
		_ = kafkaConfigMap.SetKey("security.protocol", config.Kafka.Sasl.Protocol)
		_ = kafkaConfigMap.SetKey("ssl.ca.location", config.Kafka.Capath)
	}

	if consumer, err = kafka.NewConsumer(kafkaConfigMap); err != nil {
		return nil, err
	}

	if err = consumer.SubscribeTopics(config.Kafka.Topics, nil); err != nil {
		return nil, err
	}

	return consumer, nil
}

func getHeader(msg *kafka.Message, key string) (*kafka.Header, error) {
	if msg == nil {
		return nil, fmt.Errorf("msg is nil")
	}
	for _, header := range msg.Headers {
		if header.Key == key {
			return &header, nil
		}
	}
	return nil, fmt.Errorf("could not find '%s' in message header", key)
}

// TODO Refactor to make this check dynamic so it does not need
//      to be modified if more different messages are added
func isValidEvent(event string) bool {
	switch event {
	case string(message.HdrTypeIntrospect):
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
		err   error
		event *kafka.Header
		sm    schema.SchemaMap
		s     *schema.Schema
		// b64coded   []byte
		// b64decoded []byte
	)
	if msg == nil {
		return fmt.Errorf("msg cannot be nil")
	}
	if event, err = getHeader(msg, string(message.HdrType)); err != nil {
		return fmt.Errorf("header 'event' not found: %w", err)
	}
	if !isValidEvent(string(event.Value)) {
		return fmt.Errorf("event not valid: %v", event)
	}
	if msg.TopicPartition.Topic == nil {
		return fmt.Errorf("topic cannot be nil")
	}
	topic := *msg.TopicPartition.Topic
	if sm = getSchemaMap(schemas, topic); sm == nil {
		return fmt.Errorf("topic '%s' not found in schema mapping", event.String())
	}
	if s = getSchema(sm, string(event.Value)); s == nil {
		return fmt.Errorf("schema '%s'  not found in schema mapping", event.String())
	}

	// if err = json.Unmarshal(msg.Value, &b64coded); err != nil {
	// 	return fmt.Errorf("payload unmarshall error: %w", err)
	// }
	// if b64decoded, err = b64.StdEncoding.DecodeString(string(b64coded)); err != nil {
	// 	return fmt.Errorf("decoding base64 message value: %w", err)
	// }

	// return s.ValidateBytes(b64decoded)
	return s.ValidateBytes(msg.Value)
}

func logEventMessageError(msg *kafka.Message, err error) {
	if msg == nil {
		log.Logger.Error().Msgf("msg is nil")
		return
	}
	if err == nil {
		log.Logger.Error().Msgf("err is nil")
		return
	}
	log.Logger.Error().
		Msgf("error processing event message: headers=%v; payload=%v: %w", msg.Headers, string(msg.Value), err)
}

func NewConsumerEventLoop(consumer *kafka.Consumer, handler Eventable) func() {
	var (
		err     error
		msg     *kafka.Message
		schemas schema.TopicSchemas
	)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	schemas, err = schema.LoadSchemas()
	utils.DieOnError(err)

	return func() {
		log.Logger.Info().Msg("Consumer loop awaiting to consume messages")
		for {
			// Message wait loop
			for {
				msg, err = consumer.ReadMessage(1 * time.Second)

				if err != nil {
					if err.(kafka.Error).Code() != kafka.ErrTimedOut {
						log.Logger.Error().Msgf("error awaiting to read a message: %w", err)
					}

					// TODO If moving into a multi-service design
					//      this signal handler can not be here
					//      and potentially the graceful termination
					//      would need some communication with the
					//      context (<-context.Done())
					//
					// select {
					// case <-ctx.Done():
					// 	 log.Logger.Info().Msgf("Stopping consumer event loop")
					// 	 return
					// default:
					// }
					//
					// Keep in mind that VSCode go debugger send a SIGKILL
					// signal to the debugged process, which cannot be captured
					// and no gracefull termination could happen.
					// https://github.com/golang/vscode-go/issues/120#issuecomment-1092887526
					//
					// The multi-service design could comes when adding
					// instrumentation which will public endpoints for
					// collecting prometheus metrics.
					select {
					case sig := <-sigchan:
						log.Logger.Info().Msgf("Caught signal %v: terminating\n", sig)
						return
					default:
					}

					continue
				}
				break
			}

			if err = validateMessage(schemas, msg); err != nil {
				logEventMessageError(msg, err)
				continue
			}

			// Dispatch message
			if err = handler.OnMessage(msg); err != nil {
				logEventMessageError(msg, err)
			}
		}
	}
}
