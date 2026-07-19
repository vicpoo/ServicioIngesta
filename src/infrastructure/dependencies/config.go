// src/infrastructure/dependencies/config.go
package dependencies

import (
	"fmt"
	"os"
	"strconv"
)

// Config son las variables de entorno del servicio.
type Config struct {
	Port          string
	DatabaseURL   string
	MQTTHost      string
	MQTTPort      int
	MQTTUser      string
	MQTTPass      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// RabbitMQMgmtURL habilita (si no está vacío) el drenado adicional de la
	// cola kajve_datos vía la API HTTP de administración de RabbitMQ.
	// Es un respaldo/complemento del consumidor MQTT: existe porque el
	// puerto AMQP 5672 no está expuesto en el servidor real, así que no se
	// puede usar un consumidor AMQP normal para vaciar esa cola.
	RabbitMQMgmtURL      string
	RabbitMQMgmtUser     string
	RabbitMQMgmtPass     string
	RabbitMQVHost        string
	RabbitMQQueue        string
	RabbitMQPollInterval int // segundos
}

// LoadConfig lee y valida la configuración mínima para arrancar.
func LoadConfig() (*Config, error) {
	mqttPort, err := strconv.Atoi(getEnv("MQTT_PORT", "1883"))
	if err != nil {
		return nil, fmt.Errorf("config: MQTT_PORT inválido: %w", err)
	}

	pollInterval, err := strconv.Atoi(getEnv("RABBITMQ_POLL_INTERVAL_SECONDS", "5"))
	if err != nil {
		return nil, fmt.Errorf("config: RABBITMQ_POLL_INTERVAL_SECONDS inválido: %w", err)
	}

	cfg := &Config{
		Port:          getEnv("PORT", "8001"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		MQTTHost:      os.Getenv("MQTT_HOST"),
		MQTTPort:      mqttPort,
		MQTTUser:      os.Getenv("MQTT_USER"),
		MQTTPass:      os.Getenv("MQTT_PASS"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       0,

		RabbitMQMgmtURL:      os.Getenv("RABBITMQ_MGMT_URL"),
		RabbitMQMgmtUser:     os.Getenv("RABBITMQ_MGMT_USER"),
		RabbitMQMgmtPass:     os.Getenv("RABBITMQ_MGMT_PASS"),
		RabbitMQVHost:        getEnv("RABBITMQ_VHOST", "/"),
		RabbitMQQueue:        getEnv("RABBITMQ_QUEUE", "kajve_datos"),
		RabbitMQPollInterval: pollInterval,
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: falta la variable de entorno DATABASE_URL")
	}
	if cfg.MQTTHost == "" {
		return nil, fmt.Errorf("config: falta la variable de entorno MQTT_HOST")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
