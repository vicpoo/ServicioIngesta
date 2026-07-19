//src/domain/interface.go
// Package domain define los puertos (interfaces) del hexágono: lo que la
// capa application necesita, sin saber cómo infrastructure lo implementa.
package domain

import (
	"context"

	"github.com/kajve/ingesta-iot/src/domain/entities"
)

// SensorRepository resuelve sensores físicos a partir del identificador que
// el ESP32 envía en el payload MQTT.
type SensorRepository interface {
	// GetByMacAddress busca un sensor por mac_address. Devuelve (nil, nil)
	// si no existe ningún sensor con ese identificador (dispositivo no
	// registrado), y (nil, err) solo ante un fallo real de infraestructura.
	GetByMacAddress(ctx context.Context, macAddress string) (*entities.Sensor, error)
}

// LoteRepository resuelve el lote activo ('en_proceso') de un sensor.
type LoteRepository interface {
	// GetLoteActivoPorSensor devuelve (nil, nil) si el sensor no tiene
	// ningún lote 'en_proceso' — no debería ocurrir si la migración de BD
	// (triggers de placeholder) está aplicada.
	GetLoteActivoPorSensor(ctx context.Context, sensorID int) (*entities.Lote, error)
}

// LecturaRepository persiste lecturas ambientales respetando RLS.
type LecturaRepository interface {
	// Create inserta la lectura dentro de una transacción con
	// SET LOCAL app.current_user_id = usuarioID, tal como lo exige la
	// política lecturas_por_usuario.
	Create(ctx context.Context, usuarioID int, lectura *entities.LecturaAmbiental) error
}

// RealtimePublisher entrega eventos en tiempo real hacia el canal del
// usuario dueño del dato (Redis Pub/Sub -> WebSocket Gateway).
type RealtimePublisher interface {
	PublishToUser(ctx context.Context, usuarioID int, event entities.RealtimeEvent) error
}
