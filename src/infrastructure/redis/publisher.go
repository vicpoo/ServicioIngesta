//src/infrastructure/redis/publisher.go
// Package redis implementa domain.RealtimePublisher sobre Redis Pub/Sub,
// el mismo mecanismo que ya anticipa el WebSocketManager de api-mobile
// (internal/delivery/websocket/manager.go), aunque aún no está conectado
// en su main.go.
package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/kajve/ingesta-iot/src/domain/entities"
)

// Publisher implementa domain.RealtimePublisher.
type Publisher struct {
	client *goredis.Client
}

func NewPublisher(addr, password string, db int) *Publisher {
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Publisher{client: client}
}

// Ping verifica la conexión a Redis (útil en el arranque del servicio).
func (p *Publisher) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}

func (p *Publisher) Close() error {
	return p.client.Close()
}

// PublishToUser publica en el canal "user:{usuarioID}". Confirmar con el
// equipo de api-mobile el nombre exacto del canal y esta forma de JSON antes
// de producción — hoy solo existe como comentario en su código
// (WebSocketManager.HandleUpgrade), no implementado.
func (p *Publisher) PublishToUser(ctx context.Context, usuarioID int, event entities.RealtimeEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("redis: error serializando evento: %w", err)
	}
	channel := fmt.Sprintf("user:%d", usuarioID)
	if err := p.client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("redis: error publicando en %s: %w", channel, err)
	}
	return nil
}
