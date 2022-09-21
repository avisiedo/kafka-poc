package main

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"my-test-app/pkg/config"
	"my-test-app/pkg/db"
	"my-test-app/pkg/event"
	"my-test-app/pkg/event/handler"
	"my-test-app/pkg/event/message"
	"my-test-app/pkg/event/schema"
	"my-test-app/pkg/utils"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"

	b64 "encoding/base64"

	kafka "github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
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

// func getConfig() *viper.Viper {
// 	config := viper.New()

// 	config.Set("kafka.bootstrap.servers", "localhost:9092")
// 	config.Set("kafka.request.required.acks", 1)
// 	config.Set("kafka.message.send.max.retries", 3)
// 	config.Set("kafka.group.id", "0")
// 	config.Set("kafka.auto.offset.reset", "latest")
// 	config.Set("kafka.auto.commit.interval.ms", 800)
// 	config.Set("kafka.retry.backoff.ms", 400)
// 	config.Set("kafka.topics", []string{"repos-introspect"})

// 	// TODO Assuming this will be provided into the config from managed kafka
// 	// config.Set("kafka.sasl.username", "username")
// 	// config.Set("kafka.sasl.password", "username")
// 	// config.Set("kafka.sasl.mechanism", "username")
// 	// config.Set("kafka.sasl.protocol", "username")
// 	// config.Set("kafka.sasl.capath", "username")

// 	return config
// }

func getTopics(config *config.Configuration) ([]string, error) {
	return config.Kafka.Topics, nil
}

func getKafkaReader(config *config.Configuration) (*kafka.Consumer, error) {
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

func getKafkaWriter(config *config.Configuration) (*kafka.Producer, error) {
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
		cfg *config.Configuration
		err error
	)
	cfg = config.Get()

	producer, err = getKafkaWriter(cfg)
	utils.DieOnError(err)
	consumer, err = event.NewConsumer(
		context.Background(),
		cfg,
		cfg.Kafka.Topics,
	)
	utils.DieOnError(err)

	ctx := context.Background()

	err = db.Connect()
	utils.DieOnError(err)
	dbConnector := db.DB
	handler := handler.NewIntrospectHandler(dbConnector)
	go func() {
		event.Start(ctx, cfg, dbConnector, handler)
		log.Logger.Info().Msgf("[initKafka] kafka consumer loop exited")
	}()
}

func sendMessage(context context.Context, writer *kafka.Producer, widget *Widget) error {
	var (
		err    error
		header kafka.Header
		topic  string
	)

	if widget == nil {
		return fmt.Errorf("[sendMessage] widget is nil")
	}

	// Validate Payload
	topic = schema.TopicIntrospect
	if err = schemas[topic][schema.SchemaRequestKey].Validate(widget.Payload); err != nil {
		return fmt.Errorf("[sendMessage] Payload failed schema validation: %w", err)
	}

	// Compose message
	var key string = widget.Key
	header.Key = "event"
	header.Value = []byte(widget.Header.Event)
	var msg message.IntrospectRequestMessage
	msg.B64Identity = widget.Payload.B64Identity
	msg.OrgId = widget.Payload.OrgId
	msg.RequestId = widget.Payload.RequestId
	msg.State = widget.Payload.State
	msg.Url = widget.Payload.Url

	if err = event.Produce(producer, topic, msg, key, header); err != nil {
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

func generateRequestId() string {
	var (
		randBytes []byte = make([]byte, 32)
		requestId string
		err       error
	)
	_, err = rand.Reader.Read(randBytes)
	if err != nil {
		return requestId
	}
	requestId = b64.StdEncoding.EncodeToString(randBytes)
	return requestId
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
			widget.Payload.RequestId = generateRequestId()
			if err := sendMessage(c, producer, &widget); err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}

			headerSerialized, err = json.Marshal(widget.Header)
			payloadSerialized, err = json.Marshal(widget.Payload)
			log.Printf("Header: %s", string(headerSerialized))
			log.Printf("Payload: %s", string(payloadSerialized))

			c.Header("Location", fmt.Sprintf("/kafka/%s", widget.Key))

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
		apiServer(true)
	} else {
		apiServer(false)
	}
}
