//src/infrastructure/repository/lote_repository.go
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kajve/ingesta-iot/src/domain/entities"
	"github.com/kajve/ingesta-iot/src/core"
)

// SensorRepository implementa domain.SensorRepository sobre Postgres.
type SensorRepository struct {
	db *core.DB
}

func NewSensorRepository(db *core.DB) *SensorRepository {
	return &SensorRepository{db: db}
}

// GetByMacAddress resuelve un sensor por el campo "id" que el ESP32 envía en
// el payload MQTT, haciendo match contra sensores.mac_address — el mismo
// criterio que ya usa api-mobile (SensorRepository.GetByIdentifier) en su
// flujo de vinculación por QR. No se usa id_cola_mqtt aquí: esa columna es
// la dirección de transporte, no la identidad del dispositivo.
func (r *SensorRepository) GetByMacAddress(ctx context.Context, macAddress string) (*entities.Sensor, error) {
	s := &entities.Sensor{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id_sensor, mac_address, id_cola_mqtt, tipo, estado,
		       mide_viento, mide_radiacion, mide_humedad_grano
		FROM sensores
		WHERE mac_address = $1
	`, macAddress).Scan(
		&s.ID, &s.MacAddress, &s.IDColaMQTT, &s.Tipo, &s.Estado,
		&s.MideViento, &s.MideRadiacion, &s.MideHumedadGrano,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("sensor_repository: error consultando sensor: %w", err)
	}
	return s, nil
}
