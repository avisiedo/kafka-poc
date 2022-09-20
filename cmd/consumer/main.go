package main

import (
	"context"
	config "my-test-app/pkg/config"
	"my-test-app/pkg/event"
	"my-test-app/pkg/event/handler"
	"my-test-app/pkg/event/schema"
	"my-test-app/pkg/utils"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func setupContext() context.Context {
	var (
		err error
		s   schema.TopicSchemas
	)
	s, err = schema.LoadSchemas()
	utils.DieOnError(err)
	ctx := context.WithValue(context.Background(), "schema", s)
	if ctx == nil {
		panic("[setupContext] error creating context")
	}
	return ctx
}

func GetDatabase(cfg *config.Configuration) *gorm.DB {
	// TODO Implement this function
	return nil
}

func main() {
	cfg := config.Get()
	ctx := setupContext()
	signals := make(chan os.Signal, 1)
	errors := make(chan error, 1)
	db := GetDatabase(cfg)
	wg := sync.WaitGroup{}
	handler := handler.NewIntrospectHandler(db)
	event.Start(ctx, cfg, db, errors, &wg, handler)
	select {
	case signal := <-signals:
		log.Logger.Info().Msgf("[main] Shutting down; signal = %d", signal)
		os.Exit(0)
	case error := <-errors:
		log.Logger.Error().Msgf("[main] Shutting down; error: %w", error)
		return
	}
}
