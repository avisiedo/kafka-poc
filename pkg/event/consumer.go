package event

import (
	"context"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md

func NewConsumer(ctx context.Context, config *viper.Viper, topics []string) (*kafka.Consumer, error) {

	kafkaConfigMap := &kafka.ConfigMap{
		"bootstrap.servers":        config.GetString("kafka.bootstrap.servers"),
		"group.id":                 config.GetString("kafka.group.id"),
		"auto.offset.reset":        config.GetString("kafka.auto.offset.reset"),
		"auto.commit.interval.ms":  config.GetInt("kafka.auto.commit.interval.ms"),
		"go.logs.channel.enable":   true,
		"allow.auto.create.topics": true,
	}

	if config.Get("kafka.sasl.username") != nil {
		_ = kafkaConfigMap.SetKey("sasl.username", config.GetString("kafka.sasl.username"))
		_ = kafkaConfigMap.SetKey("sasl.password", config.GetString("kafka.sasl.password"))
		_ = kafkaConfigMap.SetKey("sasl.mechanism", config.GetString("kafka.sasl.mechanism"))
		_ = kafkaConfigMap.SetKey("security.protocol", config.GetString("kafka.sasl.protocol"))
		_ = kafkaConfigMap.SetKey("ssl.ca.location", config.GetString("kafka.capath"))
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

func NewConsumerEventLoop(
	ctx context.Context,
	consumer *kafka.Consumer,
	messagePredicate KafkaMessagePredicate,
	validationPredicate KafkaMessagePredicate,
	handler func(context.Context, *kafka.Message),
	errors chan<- error,
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
					// utils.GetLogFromContext(ctx).Errorw("Error reading message from kafka", "err", err)
					log.Logger.Warn().Msgf("Error reading message from kafka: %w", err)
					errors <- err
				}

				continue
			}

			if messagePredicate != nil && !messagePredicate(msg) {
				continue
			}

			if validationPredicate != nil && !validationPredicate(msg) {
				continue
			}

			handler(ctx, msg)
		}
	}
}
