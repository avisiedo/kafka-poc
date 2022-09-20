package event

import (
	"encoding/json"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/openlyinc/pointy"
	"github.com/spf13/viper"
)

// https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md

func NewProducer(config *viper.Viper) (*kafka.Producer, error) {

	kafkaConfigMap := &kafka.ConfigMap{
		"bootstrap.servers":        config.GetString("kafka.bootstrap.servers"),
		"request.required.acks":    config.GetInt("kafka.request.required.acks"),
		"message.send.max.retries": config.GetInt("kafka.message.send.max.retries"),
		"retry.backoff.ms":         config.GetInt("kafka.retry.backoff.ms"),
	}
	if config.Get("kafka.sasl.username") != nil {
		_ = kafkaConfigMap.SetKey("sasl.username", config.GetString("kafka.sasl.username"))
		_ = kafkaConfigMap.SetKey("sasl.password", config.GetString("kafka.sasl.password"))
		_ = kafkaConfigMap.SetKey("sasl.mechanism", config.GetString("kafka.sasl.mechanism"))
		_ = kafkaConfigMap.SetKey("security.protocol", config.GetString("kafka.sasl.protocol"))
		_ = kafkaConfigMap.SetKey("ssl.ca.location", config.GetString("kafka.capath"))
	}
	producer, err := kafka.NewProducer(kafkaConfigMap)
	if err != nil {
		return nil, err
	}

	return producer, nil
}

func Produce(producer *kafka.Producer, topic string, value interface{}, key string, headers ...kafka.Header) error {
	marshalledValue, err := json.Marshal(value)
	if err != nil {
		return err
	}

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     pointy.String(topic),
			Partition: kafka.PartitionAny,
		},
		Value: marshalledValue,
		Key:   []byte(key),
	}

	for _, header := range headers {
		msg.Headers = append(msg.Headers, header)
	}

	return producer.Produce(msg, nil)
}
