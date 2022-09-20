package event

import (
	"context"
	"fmt"

	"my-test-app/pkg/event/schema"
	_ "my-test-app/pkg/utils"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
)

var defaultTopic = "__consumer_offsets"

type KafkaMessagePredicate func(msg *kafka.Message) bool

type pingable interface {
	GetMetadata(topic *string, allTopics bool, timeoutMs int) (*kafka.Metadata, error)
}

func Ping(timeout int, instances ...pingable) error {
	for _, instance := range instances {
		if _, err := instance.GetMetadata(&defaultTopic, false, timeout); err != nil {
			return err
		}
	}

	return nil
}

func Headers(keysAndValues ...string) []kafka.Header {
	if len(keysAndValues)%2 != 0 {
		panic(fmt.Sprintf("Odd number of parameters: %s", keysAndValues))
	}

	result := make([]kafka.Header, len(keysAndValues))

	for i := 0; i < len(keysAndValues)/2; i++ {
		result[i] = kafka.Header{
			Key:   keysAndValues[i*2],
			Value: []byte(keysAndValues[(i*2)+1]),
		}
	}

	return result
}

func GetHeader(msg *kafka.Message, key string) (string, error) {
	for _, value := range msg.Headers {
		if value.Key == key {
			return string(value.Value), nil
		}
	}

	return "", fmt.Errorf("Header not found: %s", key)
}

func FilterByHeaderPredicate(ctx context.Context, header string, filterVals ...string) KafkaMessagePredicate {
	return func(msg *kafka.Message) bool {
		if val, err := GetHeader(msg, header); err != nil {
			log.Logger.
				Warn().
				Err(err).
				Str("topic", *msg.TopicPartition.Topic).
				Int32("partition", msg.TopicPartition.Partition).
				Str("offset", msg.TopicPartition.Offset.String()).
				Msg("Error reading kafka message header")
			return false
		} else {
			for _, filterVal := range filterVals {
				if val == filterVal {
					return true
				}
			}
			return false
		}
	}
}

func SchemaValidationPredicate(ctx context.Context, header string, schemaMapper schema.SchemaMap) KafkaMessagePredicate {
	return func(msg *kafka.Message) bool {
		val, _ := GetHeader(msg, header)

		schema := schemaMapper[val]
		if err := schema.ValidateBytes(msg.Value); err != nil {
			log.Logger.Warn().Err(err).Msg("Error validating incoming message")
			return false
		}
		return true
	}
}
