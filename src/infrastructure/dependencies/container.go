//src/infrastructure/dependencies/container.go
// Package dependencies arma (wiring) todas las piezas del servicio:
// configuración, conexión a BD, repositorios, publisher de Redis,
// consumidor MQTT y el caso de uso de application, todo detrás de los
// puertos definidos en domain.
package dependencies

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kajve/ingesta-iot/src/application"
	"github.com/kajve/ingesta-iot/src/infrastructure/mqtt"
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
	Consumer       *mqtt.Consumer
	HTTPPoller     *rabbitmq.HTTPPoller // nil si RABBITMQ_MGMT_URL no está configurado
	IngestaService *application.IngestaService
}

// NewContainer construye el árbol de dependencias completo. Falla rápido
// (fail-fast) si algo esencial no está disponible al arrancar: BD o el
// broker MQTT. Redis es best-effort: si no está disponible, el servicio
// arranca igual y solo se pierde la notificación en tiempo real (el dato
// ya queda guardado en Postgres de todas formas). El HTTPPoller de
// RabbitMQ también es opcional: solo se activa si RABBITMQ_MGMT_URL está
// configurado en el .env.
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
		log.Printf("dependencies: Redis no disponible (%v) — el servicio sigue arrancando; las lecturas se guardarán en la BD pero no se publicarán en tiempo real hasta que Redis esté disponible", err)
	} else {
		log.Println("dependencies: conectado a Redis")
	}

	sensorRepo := repository.NewSensorRepository(db)
	loteRepo := repository.NewLoteRepository(db)
	lecturaRepo := repository.NewLecturaRepository(db)

	ingestaService := application.NewIngestaService(sensorRepo, loteRepo, lecturaRepo, redisPublisher)

	consumer, err := mqtt.NewConsumer(cfg.MQTTHost, cfg.MQTTPort, cfg.MQTTUser, cfg.MQTTPass)
	if err != nil {
		_ = redisPublisher.Close()
		db.Close()
		return nil, fmt.Errorf("dependencies: %w", err)
	}

	var httpPoller *rabbitmq.HTTPPoller
	if cfg.RabbitMQMgmtURL != "" {
		httpPoller = rabbitmq.NewHTTPPoller(
			cfg.RabbitMQMgmtURL,
			cfg.RabbitMQMgmtUser,
			cfg.RabbitMQMgmtPass,
			cfg.RabbitMQVHost,
			cfg.RabbitMQQueue,
			time.Duration(cfg.RabbitMQPollInterval)*time.Second,
		)
		log.Printf("dependencies: drenado HTTP de RabbitMQ habilitado para la cola %q", cfg.RabbitMQQueue)
	} else {
		log.Println("dependencies: RABBITMQ_MGMT_URL no configurado — no se drenará kajve_datos por HTTP, solo se consume por MQTT")
	}

	return &Container{
		Config:         cfg,
		DB:             db,
		RedisPublisher: redisPublisher,
		Consumer:       consumer,
		HTTPPoller:     httpPoller,
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