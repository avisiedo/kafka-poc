package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	kafka "github.com/segmentio/kafka-go"
)

type Widget struct {
	Name   string `form:"name"`
	Flavor string `form:"flavor"`
	Id     int64  `form:"id"`
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

func getKafkaWriter() (*kafka.Writer, error) {
	if !clowder.IsClowderEnabled() {
		return nil, errors.New("Clowder disabled")
	}
	kafkaUrl, urlError := getUrl()
	if urlError != nil {
		return nil, urlError
	}
	topic, topicError := getTopic()
	if topicError != nil {
		return nil, topicError
	}
	return &kafka.Writer{
		Addr:     kafka.TCP(kafkaUrl),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}, nil
}

func sendMessage(context context.Context, writer *kafka.Writer, id int64, name string) {
	if clowder.IsClowderEnabled() {
		msg := kafka.Message{
			Key:   []byte(fmt.Sprintf("id-%s", id)),
			Value: []byte(name),
		}
		err := writer.WriteMessages(context, msg)

		if err == nil {
			log.Println("Sent message")
		} else {
			log.Fatalln(err)
		}
	} else {
		log.Println("clowder disabled")
	}
}

func listener() {
	if !clowder.IsClowderEnabled() {
		log.Println("clowder disabled")
		return
	}
	kafkaUrl, urlError := getUrl()
	if urlError != nil {
		return
	}
	topic, topicError := getTopic()
	if topicError != nil {
		return
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
	myWidgets := make(map[int64]Widget)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	if !pingOnly {
		kafkaWriter, error := getKafkaWriter()
		if error != nil {
			log.Println("Could not initizlize kafka writer")
			log.Println(error)
		}
		kafkaWriter.BatchTimeout, error = time.ParseDuration("100ms")
		if error != nil {
			log.Println(error)
		}

		r.GET("/kafka/", func(c *gin.Context) {
			list := make([]Widget, 0, len(myWidgets))
			for _, w := range myWidgets {
				list = append(list, w)
			}
			c.JSON(http.StatusOK, list)
		})
		r.POST("/kafka/", func(c *gin.Context) {
			var widget Widget
			if c.BindJSON(&widget) == nil {
				if widget.Name == "send" {
					sendMessage(c, kafkaWriter, widget.Id, widget.Name)
				}
				log.Println(widget.Id)
				log.Println(widget.Name)
				myWidgets[widget.Id] = widget

				c.JSON(http.StatusOK, widget)
			}
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
