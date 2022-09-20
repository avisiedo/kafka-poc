package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"my-test-app/pkg/event"
	"my-test-app/pkg/event/message"
	"my-test-app/pkg/event/schema"
	"net/http"
	"os"
	"time"

	b64 "encoding/base64"

	kafka "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/spf13/viper"
)

var schemas schema.TopicSchemas
var consumer *kafka.Consumer = nil
var producer *kafka.Producer = nil

type Widget struct {
	Key     string
	Header  message.HeaderMessage
	Payload message.IntrospectRequestMessage
}

func getUrl() (string, error) {
	cfg := clowder.LoadedConfig
	if cfg.Kafka.Brokers[0].Hostname != "" {
		return fmt.Sprintf("%s:%v", cfg.Kafka.Brokers[0].Hostname, *cfg.Kafka.Brokers[0].Port), nil
	} else {
		return "", errors.New("empty name")
	}
}

func getConfig() *viper.Viper {
	config := viper.New()

	config.Set("kafka.bootstrap.servers", "localhost:9092")
	config.Set("kafka.request.required.acks", 1)
	config.Set("kafka.message.send.max.retries", 3)
	config.Set("kafka.group.id", "0")
	config.Set("kafka.auto.offset.reset", "latest")
	config.Set("kafka.auto.commit.interval.ms", 800)
	config.Set("kafka.retry.backoff.ms", 400)
	config.Set("kafka.topics", []string{"repos-introspect"})

	// TODO This will be provided into the config in managed kafka
	// config.Set("kafka.sasl.username", "username")
	// config.Set("kafka.sasl.password", "username")
	// config.Set("kafka.sasl.mechanism", "username")
	// config.Set("kafka.sasl.protocol", "username")
	// config.Set("kafka.sasl.capath", "username")

	return config
}

func getTopics(config *viper.Viper) ([]string, error) {
	return config.GetStringSlice("kafka.topics"), nil
}

func getKafkaReader(config *viper.Viper) (*kafka.Consumer, error) {
	var (
		err      error
		topics   []string
		consumer *kafka.Consumer
	)

	if topics, err = getTopics(config); err != nil {
		return nil, fmt.Errorf("[getKafkaReader] error retrieving the topic: %w", err)
	}
	if consumer, err = event.NewConsumer(context.Background(), config, topics); err != nil {
		return nil, fmt.Errorf("[getKafkaReader] error creating consumer: %w", err)
	}

	return consumer, nil
}

func getKafkaWriter(config *viper.Viper) (*kafka.Producer, error) {
	var (
		producer *kafka.Producer
		err      error
	)

	if producer, err = event.NewProducer(config); err != nil {
		return nil, err
	}
	return producer, nil
}

func initKafka() {
	var (
		config *viper.Viper
		err    error
	)
	config = getConfig()
	if consumer, err = getKafkaReader(config); err != nil {
		panic(err)
	}
	if producer, err = getKafkaWriter(config); err != nil {
		panic(err)
	}
}

func sendMessage(context context.Context, writer *kafka.Producer, widget *Widget) error {
	var (
		err              error
		header           kafka.Header
		headerSerialized []byte
		payload          []byte
		topic            string
	)

	if widget == nil {
		return fmt.Errorf("[sendMessage] widget is nil")
	}

	// Validate Header
	// if topic, err = getTopic(); err != nil {
	// 	return fmt.Errorf("[sendMessage] Erro retrieving the topic: %w", err)
	// }
	topic = schema.TopicIntrospect
	if err = schemas[topic][schema.SchemaHeaderKey].Validate(widget.Header); err != nil {
		return fmt.Errorf("[sendMessage] Header failed schema validation: %w", err)
	}
	// Validate Payload
	if err = schemas[topic][schema.SchemaRequestKey].Validate(widget.Payload); err != nil {
		return fmt.Errorf("[sendMessage] Payload failed schema validation: %w", err)
	}

	// Compose message
	if headerSerialized, err = json.Marshal(widget.Header); err != nil {
		return err
	}
	if payload, err = json.Marshal(widget.Payload); err != nil {
		return err
	}

	// TODO Future change, potential wrong usage of headers
	header.Key = "header"
	header.Value = []byte(headerSerialized)
	if err = event.Produce(producer, topic, payload, widget.Key, header); err != nil {
		return err
	}
	return nil
}

func readMessage(c context.Context, reader *kafka.Consumer) (*Widget, error) {
	type b64string struct {
		Value string `json:"string"`
	}
	var (
		err    error
		msg    *kafka.Message
		widget *Widget
		topic  string
	)
	if c == nil {
		return nil, fmt.Errorf("[readMessage] context is nil")
	}
	if msg, err = consumer.ReadMessage(1 * time.Second); err != nil {
		return nil, fmt.Errorf("[readMessage] error awaiting to read a message: %w", err)
	}

	// Handle header
	widget = &Widget{
		Key: string(msg.Key),
	}
	for _, header := range msg.Headers {
		if header.Key == "header" {
			if err = json.Unmarshal(header.Value, &widget.Header); err != nil {
				return nil, fmt.Errorf("[readMessage] error unmarshalling Header: %w", err)
			}
		}
	}
	topic = *msg.TopicPartition.Topic
	if err = schemas[topic][schema.SchemaHeaderKey].Validate(widget.Header); err != nil {
		return nil, fmt.Errorf("[readMessage] Header failed validation: %w", err)
	}

	// Handle payload

	var b64str string
	var b64decoded []byte
	if err = json.Unmarshal(msg.Value, &b64str); err != nil {
		return nil, fmt.Errorf("[readMessage] Payload unmarshall error: %w", err)
	}
	if b64decoded, err = b64.StdEncoding.DecodeString(b64str); err != nil {
		return nil, fmt.Errorf("[readMessage] Decoding base64 message value: %w", err)
	}
	if err = json.Unmarshal([]byte(b64decoded), &widget.Payload); err != nil {
		return nil, fmt.Errorf("[readMessage] Payload unmarshall error: %w", err)
	}
	if err = schemas[topic][schema.SchemaRequestKey].Validate(widget.Payload); err != nil {
		return nil, fmt.Errorf("[readMessage] Payload failed validation: %w", err)
	}

	// TODO Actions for the message comes here

	return widget, nil
}

func listener() {
	initKafka()
}

func apiServer(pingOnly bool) {
	var (
		err error
	)

	initKafka()

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	if !pingOnly {

		r.GET("/kafka", func(c *gin.Context) {
			var (
				// msg    kafka.Message
				// err    error
				widget *Widget
			)
			if widget, err = readMessage(c, consumer); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			c.JSON(http.StatusOK, widget)
		})
		r.POST("/kafka", func(c *gin.Context) {
			var (
				widget            Widget
				headerSerialized  []byte
				payloadSerialized []byte
			)
			if err := c.BindJSON(&widget.Payload); err != nil {
				c.JSON(http.StatusBadRequest, err.Error())
				return
			}

			if widget.Key == "" {
				widget.Key = uuid.NewString()
			}
			widget.Header.Event = "Request"
			widget.Payload.RequestId = fmt.Sprintf("%s", widget.Header.Uuid)
			if err := sendMessage(c, producer, &widget); err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}

			headerSerialized, err = json.Marshal(widget.Header)
			payloadSerialized, err = json.Marshal(widget.Payload)
			log.Printf("Header: %s", string(headerSerialized))
			log.Printf("Payload: %s", string(payloadSerialized))

			c.Header("Location", fmt.Sprintf("/kafka/%s", widget.Header.Uuid))

			c.JSON(http.StatusAccepted, widget)
		})
	}
	r.Run(":8000") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func main() {
	var err error
	if schemas, err = schema.LoadSchemas(); err != nil {
		err = fmt.Errorf("[main] error at UnmarshallSchemas: %w", err)
		panic(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "listener" {
		go func() {
			apiServer(true)
		}()
		listener()
	} else {
		apiServer(false)
	}
}
