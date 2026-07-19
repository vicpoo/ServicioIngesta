//src/core/db.go
// Package core contiene la conexión a la base de datos y el patrón de RLS
// compartido por todos los repositorios de infrastructure.
package core

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB envuelve el pool de conexiones de Postgres.
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB abre el pool de conexiones y verifica que responda antes de
// devolver el control (fail-fast si la BD no está disponible al arrancar).
func NewDB(ctx context.Context, connString string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("core: error interpretando DATABASE_URL: %w", err)
	}
	// Pool modesto: Ingesta es un solo proceso consumiendo una cola con
	// prefetch limitado (ver infrastructure/rabbitmq), no necesita tantas
	// conexiones concurrentes como una API HTTP.
	cfg.MaxConns = 10
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("core: error creando el pool de conexiones: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("core: error conectando a la base de datos: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close libera el pool de conexiones.
func (d *DB) Close() {
	d.Pool.Close()
}

// WithUserContext ejecuta fn dentro de una transacción con
// app.current_user_id fijado como configuración LOCAL a esa transacción
// (se resetea solo al hacer commit/rollback). Esto es lo que exige la
// política RLS lecturas_por_usuario sobre lecturas_ambientales, y replica
// el mismo patrón que ya usa api-mobile (PostgresDB.BeginTx), pero con
// set_config(..., true) en vez de un SET con interpolación de string, para
// que el valor viaje como parámetro real y quede acotado a la transacción
// sin filtrarse a otras consultas que reutilicen la misma conexión del pool.
func (d *DB) WithUserContext(ctx context.Context, usuarioID int, fn func(tx pgx.Tx) error) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("core: error iniciando transacción: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op si ya se hizo commit

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_user_id', $1, true)`, strconv.Itoa(usuarioID)); err != nil {
		return fmt.Errorf("core: error fijando app.current_user_id: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("core: error confirmando transacción: %w", err)
	}
	return nil
}
