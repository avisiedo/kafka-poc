package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	kafka "github.com/segmentio/kafka-go"
)

type MessageUpdateStatus struct {
	RepositoryURL   string `json:"url"`
	RepositoryCount int    `json:"count"`
	Status          string `json:"status"`
	Comment         string `json:"comment"`
}

type MessageKey struct {
	Id int64 `json:"id"`
}
type Widget struct {
	Key     MessageKey
	Payload MessageUpdateStatus
}

func getUrl() (string, error) {
	cfg := clowder.LoadedConfig
	if cfg.Kafka.Brokers[0].Hostname != "" {
		return fmt.Sprintf("%s:%v", cfg.Kafka.Brokers[0].Hostname, *cfg.Kafka.Brokers[0].Port), nil
	} else {
		return "", errors.New("empty name")
	}
}

func getTopic() (string, error) {
	topic := "foo"
	for _, topicConfig := range clowder.KafkaTopics {
		topic = topicConfig.Name
	}
	if topic == "" {
		return "", errors.New("empty name")
	} else {
		return topic, nil
	}

}

func getKafkaReader() (*kafka.Reader, error) {
	var (
		err      error
		kafkaUrl string
		topic    string
	)
	if clowder.IsClowderEnabled() {
		if kafkaUrl, err = getUrl(); err != nil {
			return nil, err
		}
		log.Printf("kafkaUrl = '%s'", kafkaUrl)
		if topic, err = getTopic(); err != nil {
			return nil, err
		}
		log.Printf("topic = '%s'", topic)
	} else {
		kafkaUrl = "localhost:9092"
		topic = "repos.created"
	}

	config := kafka.ReaderConfig{
		Topic:    topic,
		Brokers:  []string{kafkaUrl},
		MaxWait:  500 * time.Millisecond,
		MinBytes: 1,
		MaxBytes: 100,
	}

	return kafka.NewReader(config), nil
}

func getKafkaWriter() (*kafka.Writer, error) {
	if !clowder.IsClowderEnabled() {
		log.Println("clowder disabled")
		kafkaUrl := "localhost:9092"
		topic := "repos.created"
		log.Printf("kafkaUrl = '%s'", kafkaUrl)
		log.Printf("topic = '%s'", topic)
		return &kafka.Writer{
			Addr:     kafka.TCP(kafkaUrl),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		}, nil
	}
	kafkaUrl, urlError := getUrl()
	if urlError != nil {
		return nil, urlError
	}
	log.Printf("kafkaUrl = '%s'", kafkaUrl)
	topic, topicError := getTopic()
	if topicError != nil {
		return nil, topicError
	}
	log.Printf("topic = '%s'", topic)
	return &kafka.Writer{
		Addr:     kafka.TCP(kafkaUrl),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}, nil
}

func sendMessage(context context.Context, writer *kafka.Writer, widget *Widget) error {
	var (
		err     error
		header  []byte
		payload []byte
	)
	// if clowder.IsClowderEnabled() {
	// msg := kafka.Message{
	// 	Key:   []byte(fmt.Sprintf("id-%d", id)),
	// 	Value: []byte(name),
	// }
	if widget == nil {
		return fmt.Errorf("widget is nil")
	}
	if header, err = json.Marshal(widget.Key); err != nil {
		return err
	}
	if payload, err = json.Marshal(widget.Payload); err != nil {
		return err
	}
	msg := kafka.Message{
		Key:   header,
		Value: payload,
	}
	if err = writer.WriteMessages(context, msg); err != nil {
		return err
	}

	return nil

	// } else {
	// 	log.Println("clowder disabled")
	// }
}

func listener() {
	var (
		kafkaUrl string
		topic    string
		err      error
	)
	if !clowder.IsClowderEnabled() {
		log.Println("clowder disabled")
		kafkaUrl = "localhost:9092"
		topic = "repos.created"
	} else {
		if kafkaUrl, err = getUrl(); err != nil {
			return
		}
		if topic, err = getTopic(); err != nil {
			return
		}
	}
	partition := 0
	conn, err := kafka.DialLeader(context.Background(), "tcp", kafkaUrl, topic, partition)
	if err != nil {
		log.Fatal("failed to dial leader:", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	batch := conn.ReadBatch(10e3, 1e6)
	b := make([]byte, 10e3) // 10KB max per message
	for {
		n, err := batch.Read(b)
		if err != nil {
			break
		}
		fmt.Println(string(b[:n]))
	}
}

func apiServer(pingOnly bool) {
	var (
		err         error
		kafkaWriter *kafka.Writer
		kafkaReader *kafka.Reader
	)
	myWidgets := make(map[int64]Widget)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	if !pingOnly {
		if kafkaWriter, err = getKafkaWriter(); err != nil {
			log.Println("Could not initialize kafka writer")
			panic(err)
		}
		if kafkaWriter.BatchTimeout, err = time.ParseDuration("100ms"); err != nil {
			panic(err)
		}

		if kafkaReader, err = getKafkaReader(); err != nil {
			panic(err)
		}

		r.GET("/kafka/", func(c *gin.Context) {
			var (
				msg    kafka.Message
				err    error
				widget Widget
			)
			if msg, err = kafkaReader.ReadMessage(c); err != nil {
				c.AbortWithError(http.StatusNoContent, err)
				return
			}
			if err = json.Unmarshal(msg.Key, &widget.Key); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			if err = json.Unmarshal(msg.Value, &widget.Payload); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			c.JSON(http.StatusOK, widget)
		})
		r.POST("/kafka/", func(c *gin.Context) {
			var widget Widget
			if err := c.BindJSON(&widget.Payload); err != nil {
				c.JSON(http.StatusBadRequest, err.Error())
				return
			}
			if widget.Key.Id == 0 {
				widget.Key.Id = int64(rand.Intn(10000))
			}
			if widget.Payload.Comment == "" {
				widget.Payload.Comment = fmt.Sprintf("message id = %d", widget.Key.Id)
			}
			sendMessage(c, kafkaWriter, &widget)
			log.Printf("Id = %d", widget.Key.Id)
			log.Printf("Payload: { url: '%s', count: %d, status: '%s' }", widget.Payload.RepositoryURL, widget.Payload.RepositoryCount, widget.Payload.Status)
			myWidgets[widget.Key.Id] = widget
			c.Header("Location", fmt.Sprintf("/kafka/%d", widget.Key.Id))

			c.JSON(http.StatusAccepted, widget)
		})
		r.GET("/kafka/:id", func(c *gin.Context) {
			id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
			widget, found := myWidgets[id]
			if found {
				c.JSON(http.StatusOK, widget)
			} else {
				c.String(404, "Not Found")
			}
		})
	}
	r.Run(":8000") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "listener" {
		go func() {
			apiServer(true)
		}()
		listener()
	} else {
		apiServer(false)
	}
}
