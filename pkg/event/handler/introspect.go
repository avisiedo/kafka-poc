package handler

import (
	"context"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type IntrospectHandler struct {
	Tx *gorm.DB
}

func (h *IntrospectHandler) OnMessage(ctx context.Context, msg *kafka.Message) error {
	log.Debug().Msg("OnMessage was called")
	return nil
}

func NewIntrospectHandler(db *gorm.DB) *IntrospectHandler {
	return &IntrospectHandler{
		Tx: db,
	}
}
