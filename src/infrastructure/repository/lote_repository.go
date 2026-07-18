package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kajve/ingesta-iot/src/domain/entities"
	"github.com/kajve/ingesta-iot/src/core"
)

// LoteRepository implementa domain.LoteRepository sobre Postgres.
type LoteRepository struct {
	db *core.DB
}

func NewLoteRepository(db *core.DB) *LoteRepository {
	return &LoteRepository{db: db}
}

// GetLoteActivoPorSensor busca el lote 'en_proceso' de un sensor. Gracias al
// índice único parcial uq_lotes_sensor_en_proceso (ver migración de BD),
// nunca hay más de una fila: o es el placeholder del usuario reservado
// (id_usuario = 10, mientras nadie ha reclamado el sensor), o es el del
// productor real que ya lo vinculó desde la app móvil.
func (r *LoteRepository) GetLoteActivoPorSensor(ctx context.Context, sensorID int) (*entities.Lote, error) {
	l := &entities.Lote{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id_lote, id_usuario, id_sensor, estado
		FROM lotes_cafe
		WHERE id_sensor = $1 AND estado = 'en_proceso'
		LIMIT 1
	`, sensorID).Scan(&l.ID, &l.UsuarioID, &l.SensorID, &l.Estado)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lote_repository: error consultando lote activo: %w", err)
	}
	return l, nil
}
