// Package application contiene los casos de uso: la orquestación del
// hexágono, sin saber nada de Postgres, RabbitMQ ni Redis en concreto
// (solo conoce los puertos definidos en domain).
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/kajve/ingesta-iot/src/domain"
	"github.com/kajve/ingesta-iot/src/domain/entities"
)

// IngestaService es el caso de uso central: valida, normaliza, resuelve al
// propietario, persiste y publica en tiempo real cada lectura del ESP32.
type IngestaService struct {
	sensores domain.SensorRepository
	lotes    domain.LoteRepository
	lecturas domain.LecturaRepository
	realtime domain.RealtimePublisher
}

func NewIngestaService(
	sensores domain.SensorRepository,
	lotes domain.LoteRepository,
	lecturas domain.LecturaRepository,
	realtime domain.RealtimePublisher,
) *IngestaService {
	return &IngestaService{
		sensores: sensores,
		lotes:    lotes,
		lecturas: lecturas,
		realtime: realtime,
	}
}

func (s *IngestaService) HandleMessage(ctx context.Context, rawBody []byte) error {
	var payload entities.RawSensorPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		log.Printf("ingesta: payload inválido, se descarta: %v — body=%s", err, string(rawBody))
		return nil
	}
	if payload.ID == "" {
		log.Printf("ingesta: payload sin campo \"id\", se descarta: %s", string(rawBody))
		return nil
	}

	sensor, err := s.sensores.GetByMacAddress(ctx, payload.ID)
	if err != nil {
		return fmt.Errorf("ingesta: error resolviendo sensor: %w", err)
	}
	if sensor == nil {
		log.Printf("ingesta: dispositivo no registrado (id=%q), se descarta", payload.ID)
		return nil
	}

	lote, err := s.lotes.GetLoteActivoPorSensor(ctx, sensor.ID)
	if err != nil {
		return fmt.Errorf("ingesta: error resolviendo lote activo: %w", err)
	}
	if lote == nil {
		// No debería pasar si la migración de BD (triggers de placeholder)
		// está aplicada: todo sensor registrado siempre debería tener un
		// lote 'en_proceso' (el del usuario reservado o el del productor
		// real). Si aparece, es señal de que la migración no se aplicó.
		log.Printf("ingesta: sensor id_sensor=%d (mac=%q) sin lote 'en_proceso' — revisar migración de BD", sensor.ID, sensor.MacAddress)
		return nil
	}

	lectura := normalize(sensor, &payload)
	lectura.SensorID = sensor.ID
	lectura.LoteID = lote.ID

	if err := s.lecturas.Create(ctx, lote.UsuarioID, lectura); err != nil {
		return fmt.Errorf("ingesta: error persistiendo lectura (lote_id=%d): %w", lote.ID, err)
	}

	event := entities.RealtimeEvent{
		Tipo:      "osil.data.updated",
		LoteID:    lote.ID,
		SensorID:  sensor.ID,
		Timestamp: lectura.Timestamp,
		Lectura: entities.LecturaAmbientalDTO{
			Temperatura:      lectura.Temperatura,
			Humedad:          lectura.Humedad,
			TemperaturaGrano: lectura.TemperaturaGrano,
			Luz:              lectura.Luz,
			Lluvia:           lectura.Lluvia,
			HumedadGrano:     lectura.HumedadGrano,
			PresionHpa:       lectura.PresionHpa,
			AltitudM:         lectura.AltitudM,
		},
	}
	if err := s.realtime.PublishToUser(ctx, lote.UsuarioID, event); err != nil {
		// El dato ya quedó persistido; un fallo aquí solo pierde la
		// notificación en tiempo real de este evento puntual, no amerita
		// reintentar (y duplicar) todo el mensaje.
		log.Printf("ingesta: error publicando evento en tiempo real (lote_id=%d): %v", lote.ID, err)
	}

	return nil
}

func normalize(sensor *entities.Sensor, p *entities.RawSensorPayload) *entities.LecturaAmbiental {
	l := &entities.LecturaAmbiental{
		Temperatura:      p.TempAmbienteC,
		TemperaturaGrano: p.TempGranoC,
		Luz:              p.LuzLux,
		PresionHpa:       p.PresionHpa,
		AltitudM:         p.AltitudM,
		Timestamp:        time.Now().UTC(),
	}

	if p.LluviaAnalog != nil {
		l.Lluvia = normalizeLluvia(*p.LluviaAnalog)
	}
	if sensor.MideHumedadGrano && p.HumedadGrano != nil {
		l.HumedadGrano = calibrateHumedadGrano(*p.HumedadGrano)
	}

	return l
}

func normalizeLluvia(raw int) *float64 {
	if raw < 0 {
		raw = 0
	}
	if raw > 4095 {
		raw = 4095
	}
	v := float64(4095-raw) / 4095.0
	return &v
}

func calibrateHumedadGrano(raw float64) *float64 {
	_ = raw
	return nil
}