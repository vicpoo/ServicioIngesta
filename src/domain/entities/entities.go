//src/domain/entities/entities.go
// Package entities contiene los modelos de dominio del servicio de Ingesta.
package entities

import "time"

// RawSensorPayload es el mensaje crudo tal como lo publica el ESP32 en la
// cola de RabbitMQ. Los campos son punteros porque el ESP32 puede omitir
// alguno según qué sensores tenga habilitados (ver Sensor.MideViento, etc.).
//
// Ejemplo real:
//
//	{"id":"kajve-CA689C","temp_ambiente_c":30.42,"presion_hpa":952.445,
//	 "altitud_m":519.007,"temp_grano_c":32.1875,"luz_lux":10.83333,
//	 "lluvia_analog":4095,"lluvia_detectada":false,"humedad_grano":4095}
type RawSensorPayload struct {
	ID              string          `json:"id"`
	TempAmbienteC   *float64        `json:"temp_ambiente_c"`
	PresionHpa      *float64        `json:"presion_hpa"`
	AltitudM        *float64        `json:"altitud_m"`
	TempGranoC      *float64        `json:"temp_grano_c"`
	LuzLux          *float64        `json:"luz_lux"`
	LluviaAnalog    *int            `json:"lluvia_analog"`
	LluviaDetectada *bool           `json:"lluvia_detectada"`
	HumedadGrano    *float64        `json:"humedad_grano"`
	EstadoSensores  *EstadoSensores `json:"estado_sensores,omitempty"`
}

// EstadoSensores refleja qué sensores físicos del ESP32 están conectados y
// funcionando al momento de esta lectura (bmp280, ds18b20, bh1750 se
// autodetectan; fc37 y humedad_suelo son analógicos puros y el firmware
// siempre los manda en true porque no hay forma de saber si siguen
// conectados). Es solo informativo: nunca se persiste en
// lecturas_ambientales — Ingesta únicamente lo reenvía tal cual por Redis
// para que la app lo muestre en tiempo real.
type EstadoSensores struct {
	Bmp280       *bool `json:"bmp280,omitempty"`
	Ds18b20      *bool `json:"ds18b20,omitempty"`
	Bh1750       *bool `json:"bh1750,omitempty"`
	Fc37         *bool `json:"fc37,omitempty"`
	HumedadSuelo *bool `json:"humedad_suelo,omitempty"`
}

// Sensor refleja la tabla sensores. El identificador que el ESP32 envía como
// "id" en el payload se resuelve contra MacAddress (mismo criterio que ya
// usa api-mobile en su flujo de vinculación QR); IDColaMQTT es la dirección
// de transporte (a qué cola/tópico está asociado), un dato distinto.
type Sensor struct {
	ID               int
	MacAddress       string
	IDColaMQTT       string
	Tipo             string
	Estado           string
	MideViento       bool
	MideRadiacion    bool
	MideHumedadGrano bool
}

// Lote refleja la tabla lotes_cafe, solo los campos que Ingesta necesita
// para resolver al propietario de una lectura.
type Lote struct {
	ID        int
	UsuarioID int
	SensorID  *int
	Estado    string
}

// LecturaAmbiental es una lectura ya normalizada y calibrada, lista para
// persistir en lecturas_ambientales.
//
// Alineado con la migración de BD: se eliminaron humedad, velocidad_viento
// y radiacion_solar (ya no se miden), y la columna "lluvia" (numeric 0-1)
// se reemplazó por lluvia_analog (lectura cruda del ADC, smallint) y
// lluvia_detectada (boolean). humedad_grano también pasó a smallint porque
// llega como lectura cruda del ADC (0-4095), no como porcentaje.
type LecturaAmbiental struct {
	SensorID         int
	LoteID           int
	Temperatura      *float64
	TemperaturaGrano *float64
	Luz              *float64
	LluviaAnalog     *int16
	LluviaDetectada  *bool
	HumedadGrano     *int16
	PresionHpa       *float64
	AltitudM         *float64
	Timestamp        time.Time
}

// LecturaAmbientalDTO es la forma de una lectura al viajar por Redis Pub/Sub
// hacia el WebSocket Gateway. omitempty para no mandar campos que este
// sensor no mide.
type LecturaAmbientalDTO struct {
	Temperatura      *float64        `json:"temperatura,omitempty"`
	TemperaturaGrano *float64        `json:"temperatura_grano,omitempty"`
	Luz              *float64        `json:"luz,omitempty"`
	LluviaAnalog     *int16          `json:"lluvia_analog,omitempty"`
	LluviaDetectada  *bool           `json:"lluvia_detectada,omitempty"`
	HumedadGrano     *int16          `json:"humedad_grano,omitempty"`
	PresionHpa       *float64        `json:"presion_hpa,omitempty"`
	AltitudM         *float64        `json:"altitud_m,omitempty"`
	// EstadoSensores viaja tal cual llegó del ESP32 (ver comentario en
	// RawSensorPayload/EstadoSensores): no sale de ninguna columna de
	// Postgres, es un passthrough directo del payload crudo.
	EstadoSensores *EstadoSensores `json:"estado_sensores,omitempty"`
}

// RealtimeEvent es lo que Ingesta publica en el canal Redis user:{usuarioID}
// para que el WebSocket Gateway lo reenvíe únicamente al dueño del osil.
// Confirmar con el equipo de api-mobile el nombre exacto del canal y esta
// forma de JSON antes de producción (su WebSocketManager aún no está
// conectado en main.go).
type RealtimeEvent struct {
	Tipo      string              `json:"tipo"`
	LoteID    int                 `json:"lote_id"`
	SensorID  int                 `json:"sensor_id"`
	Lectura   LecturaAmbientalDTO `json:"lectura"`
	Timestamp time.Time           `json:"timestamp"`
}