package main

import (
	"context"
	config "my-test-app/pkg/config"
	"my-test-app/pkg/db"
	"my-test-app/pkg/event"
	"my-test-app/pkg/event/handler"

	"gorm.io/gorm"
)

func GetDatabase(cfg *config.Configuration) *gorm.DB {
	db.Connect()
	return db.DB
}

func main() {
	cfg := config.Get()
	ctx := context.Background()
	db := GetDatabase(cfg)
	handler := handler.NewIntrospectHandler(db)
	event.Start(ctx, cfg, db, handler)
}
