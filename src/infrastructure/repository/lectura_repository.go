//src/infrastructure/repository/lectura_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kajve/ingesta-iot/src/core"
	"github.com/kajve/ingesta-iot/src/domain/entities"
)

// LecturaRepository implementa domain.LecturaRepository sobre Postgres.
type LecturaRepository struct {
	db *core.DB
}

func NewLecturaRepository(db *core.DB) *LecturaRepository {
	return &LecturaRepository{db: db}
}

// Create inserta la lectura dentro de una transacción con
// SET LOCAL app.current_user_id (vía core.DB.WithUserContext), tal como lo
// exige la política RLS lecturas_por_usuario. usuarioID puede ser el del
// productor real o el del usuario reservado (id_usuario = 10) mientras el
// sensor no ha sido reclamado — en ambos casos la política se cumple igual.
//
// Columnas alineadas con la migración de BD: humedad, velocidad_viento,
// radiacion_solar y la vieja columna lluvia ya no existen; se insertan en
// su lugar lluvia_analog y lluvia_detectada.
func (r *LecturaRepository) Create(ctx context.Context, usuarioID int, l *entities.LecturaAmbiental) error {
	return r.db.WithUserContext(ctx, usuarioID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO lecturas_ambientales
				(id_sensor, id_lote, temperatura, temperatura_grano, luz,
				 lluvia_analog, lluvia_detectada, humedad_grano,
				 presion_hpa, altitud_m, "timestamp")
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`,
			l.SensorID, l.LoteID, l.Temperatura, l.TemperaturaGrano, l.Luz,
			l.LluviaAnalog, l.LluviaDetectada, l.HumedadGrano,
			l.PresionHpa, l.AltitudM, l.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("lectura_repository: error insertando lectura: %w", err)
		}
		return nil
	})
}