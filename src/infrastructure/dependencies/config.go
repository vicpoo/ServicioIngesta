package dependencies

import (
	"fmt"
	"os"
)

// Config son las variables de entorno del servicio. Se cargan desde el
// entorno del proceso (main.go intenta leer un .env primero con godotenv,
// si existe).
type Config struct {
	Port          string
	DatabaseURL   string
	AMQPURL       string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

// LoadConfig lee y valida la configuración mínima para arrancar.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8001"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AMQPURL:       os.Getenv("AMQP_URL"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       0,
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: falta la variable de entorno DATABASE_URL")
	}
	if cfg.AMQPURL == "" {
		return nil, fmt.Errorf("config: falta la variable de entorno AMQP_URL")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
