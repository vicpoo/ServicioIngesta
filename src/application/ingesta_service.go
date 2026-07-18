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

// HandleMessage procesa un mensaje crudo de la cola kajve_datos de punta a
// punta: validar -> normalizar/calibrar -> resolver propietario -> persistir
// -> publicar en tiempo real.
//
// Devuelve error SOLO ante fallos transitorios que ameriten reintento (BD o
// Redis caídos). Un payload inválido o un sensor desconocido se consideran
// "resueltos" (se loguean y se descartan con error nil), para no atascar la
// cola reintentando mensajes que nunca van a poder procesarse — ese es el
// criterio de dead-letter implícito hasta que se configure una DLX real en
// RabbitMQ (ver Fase 6 del plan).
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

// normalize traduce el payload crudo del ESP32 (patrón Adapter) a una
// lectura lista para persistir, usando las banderas mide_viento /
// mide_radiacion / mide_humedad_grano del sensor para saber qué campos
// esperar en vez de asumir que todos los dispositivos envían lo mismo.
func normalize(sensor *entities.Sensor, p *entities.RawSensorPayload) *entities.LecturaAmbiental {
	l := &entities.LecturaAmbiental{
		Temperatura:      p.TempAmbienteC,
		TemperaturaGrano: p.TempGranoC,
		Luz:              p.LuzLux,
		PresionHpa:       p.PresionHpa,
		AltitudM:         p.AltitudM,
		Timestamp:        time.Now().UTC(),
		// Humedad ambiental (relativa), VelocidadViento y RadiacionSolar
		// quedan nil -> NULL: este ESP32 no las mide (usa un sensor de
		// presión/altitud, no uno con humedad, y no tiene anemómetro ni
		// piranómetro). No se inventan valores.
	}

	if p.LluviaAnalog != nil {
		l.Lluvia = normalizeLluvia(*p.LluviaAnalog)
	}
	if sensor.MideHumedadGrano && p.HumedadGrano != nil {
		l.HumedadGrano = calibrateHumedadGrano(*p.HumedadGrano)
	}

	return l
}

// normalizeLluvia convierte el valor crudo de ADC (0-4095) a un valor
// normalizado 0-1, donde 0 = seco y 1 = lluvia máxima detectada.
//
// HIPÓTESIS DE CALIBRACIÓN, pendiente de confirmar con quien programó el
// firmware (ver plan de Ingesta, Sección 3.1): en el payload de referencia,
// lluvia_analog=4095 corresponde a lluvia_detectada=false (seco), por lo que
// se asume que valores altos de ADC = seco.
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

// calibrateHumedadGrano debería convertir el valor crudo de ADC del sensor
// capacitivo a un porcentaje 0-100. PENDIENTE: no hay fórmula de calibración
// confirmada todavía (requiere los puntos de calibración seco/húmedo del
// sensor físico); por ahora se deja el valor crudo sin transformar, para no
// insertar una calibración inventada. Ajustar esta función en cuanto exista
// la fórmula real — está aislada aquí a propósito para que ese cambio no
// toque el resto del pipeline.
func calibrateHumedadGrano(raw float64) *float64 {
	v := raw
	return &v
}
