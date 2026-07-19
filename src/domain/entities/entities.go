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
	ID              string   `json:"id"`
	TempAmbienteC   *float64 `json:"temp_ambiente_c"`
	PresionHpa      *float64 `json:"presion_hpa"`
	AltitudM        *float64 `json:"altitud_m"`
	TempGranoC      *float64 `json:"temp_grano_c"`
	LuzLux          *float64 `json:"luz_lux"`
	LluviaAnalog    *int     `json:"lluvia_analog"`
	LluviaDetectada *bool    `json:"lluvia_detectada"`
	HumedadGrano    *float64 `json:"humedad_grano"`
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
type LecturaAmbiental struct {
	SensorID         int
	LoteID           int
	Temperatura      *float64
	Humedad          *float64
	TemperaturaGrano *float64
	Luz              *float64
	Lluvia           *float64
	HumedadGrano     *float64
	VelocidadViento  *float64
	RadiacionSolar   *float64
	PresionHpa       *float64
	AltitudM         *float64
	Timestamp        time.Time
}

// LecturaAmbientalDTO es la forma de una lectura al viajar por Redis Pub/Sub
// hacia el WebSocket Gateway. omitempty para no mandar campos que este
// sensor no mide.
type LecturaAmbientalDTO struct {
	Temperatura      *float64 `json:"temperatura,omitempty"`
	Humedad          *float64 `json:"humedad,omitempty"`
	TemperaturaGrano *float64 `json:"temperatura_grano,omitempty"`
	Luz              *float64 `json:"luz,omitempty"`
	Lluvia           *float64 `json:"lluvia,omitempty"`
	HumedadGrano     *float64 `json:"humedad_grano,omitempty"`
	PresionHpa       *float64 `json:"presion_hpa,omitempty"`
	AltitudM         *float64 `json:"altitud_m,omitempty"`
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
