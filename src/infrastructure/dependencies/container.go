// Package dependencies arma (wiring) todas las piezas del servicio:
// configuración, conexión a BD, repositorios, publisher de Redis,
// consumidor de RabbitMQ y el caso de uso de application, todo detrás de
// los puertos definidos en domain.
package dependencies

import (
	"context"
	"fmt"

	"github.com/kajve/ingesta-iot/src/application"
	"github.com/kajve/ingesta-iot/src/infrastructure/rabbitmq"
	infraredis "github.com/kajve/ingesta-iot/src/infrastructure/redis"
	"github.com/kajve/ingesta-iot/src/infrastructure/repository"
	"github.com/kajve/ingesta-iot/src/core"
)

// Container agrupa todas las dependencias ya construidas, listas para que
// main.go las use.
type Container struct {
	Config         *Config
	DB             *core.DB
	RedisPublisher *infraredis.Publisher
	Consumer       *rabbitmq.Consumer
	IngestaService *application.IngestaService
}

// NewContainer construye el árbol de dependencias completo. Falla rápido
// (fail-fast) si algo esencial no está disponible al arrancar: BD, Redis o
// RabbitMQ.
func NewContainer(ctx context.Context) (*Container, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	db, err := core.NewDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("dependencies: %w", err)
	}

	redisPublisher := infraredis.NewPublisher(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err := redisPublisher.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("dependencies: error conectando a Redis: %w", err)
	}

	sensorRepo := repository.NewSensorRepository(db)
	loteRepo := repository.NewLoteRepository(db)
	lecturaRepo := repository.NewLecturaRepository(db)

	ingestaService := application.NewIngestaService(sensorRepo, loteRepo, lecturaRepo, redisPublisher)

	consumer, err := rabbitmq.NewConsumer(cfg.AMQPURL)
	if err != nil {
		_ = redisPublisher.Close()
		db.Close()
		return nil, fmt.Errorf("dependencies: %w", err)
	}

	return &Container{
		Config:         cfg,
		DB:             db,
		RedisPublisher: redisPublisher,
		Consumer:       consumer,
		IngestaService: ingestaService,
	}, nil
}

// Close libera todos los recursos en orden inverso a como se crearon.
func (c *Container) Close() {
	if c.Consumer != nil {
		_ = c.Consumer.Close()
	}
	if c.RedisPublisher != nil {
		_ = c.RedisPublisher.Close()
	}
	if c.DB != nil {
		c.DB.Close()
	}
}
