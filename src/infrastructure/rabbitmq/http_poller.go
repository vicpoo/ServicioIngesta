// Este archivo SÍ está activo (a diferencia de consumer.go en este mismo
// paquete, que quedó descontinuado). Drena la cola kajve_datos usando la
// API HTTP de administración de RabbitMQ (la misma que usa la interfaz web
// de administración), en vez de AMQP puro por el puerto 5672 — ese puerto
// no está expuesto en el servidor real, pero la API HTTP de administración
// sí (vía el dominio con proxy inverso que ya usa el panel web).
//
// Importante: la API HTTP de get-mensajes de RabbitMQ NO es un mecanismo de
// entrega confiable (no hay ack en dos fases: al pedir un mensaje con
// ackmode=ack_requeue_false, RabbitMQ ya lo da por entregado y lo borra de
// la cola en ese mismo instante). Si algo falla en el procesamiento después
// de leerlo, el mensaje ya no vuelve. Es aceptable como solución mientras el
// puerto AMQP 5672 no esté disponible, pero no reemplaza un consumidor AMQP
// real a largo plazo.
package rabbitmq

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

// HTTPPoller drena periódicamente una cola de RabbitMQ vía su API HTTP de
// administración.
type HTTPPoller struct {
	baseURL  string
	user     string
	pass     string
	vhost    string
	queue    string
	count    int
	interval time.Duration
	client   *http.Client
}

// NewHTTPPoller construye el poller. baseURL debe ser el origen del panel de
// administración, por ejemplo "https://rabbitmqtt.dnc-ed-denz.shop" (sin
// slash final, sin /api).
func NewHTTPPoller(baseURL, user, pass, vhost, queue string, interval time.Duration) *HTTPPoller {
	return &HTTPPoller{
		baseURL:  baseURL,
		user:     user,
		pass:     pass,
		vhost:    vhost,
		queue:    queue,
		count:    50,
		interval: interval,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

type getMessagesRequest struct {
	Count    int    `json:"count"`
	Ackmode  string `json:"ackmode"`
	Encoding string `json:"encoding"`
}

type rabbitMessage struct {
	Payload         string `json:"payload"`
	PayloadEncoding string `json:"payload_encoding"`
	RoutingKey      string `json:"routing_key"`
}

// Run bloquea hasta que ctx se cancele, drenando la cola cada `interval`.
func (p *HTTPPoller) Run(ctx context.Context, handler MessageHandler) {
	log.Printf("rabbitmq-http-poller: iniciado - drenando %q cada %s via %s", p.queue, p.interval, p.baseURL)

	// Primer drenado inmediato, sin esperar al primer tick, para vaciar el
	// backlog acumulado en cuanto arranca el servicio.
	if err := p.drainOnce(ctx, handler); err != nil {
		log.Printf("rabbitmq-http-poller: error drenando %q: %v", p.queue, err)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("rabbitmq-http-poller: detenido")
			return
		case <-ticker.C:
			if err := p.drainOnce(ctx, handler); err != nil {
				log.Printf("rabbitmq-http-poller: error drenando %q: %v", p.queue, err)
			}
		}
	}
}

func (p *HTTPPoller) drainOnce(ctx context.Context, handler MessageHandler) error {
	vhostEsc := url.PathEscape(p.vhost)
	queueEsc := url.PathEscape(p.queue)
	endpoint := fmt.Sprintf("%s/api/queues/%s/%s/get", p.baseURL, vhostEsc, queueEsc)

	reqBody, err := json.Marshal(getMessagesRequest{
		Count:    p.count,
		Ackmode:  "ack_requeue_false",
		Encoding: "auto",
	})
	if err != nil {
		return fmt.Errorf("armando request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("creando request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(p.user, p.pass)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("llamando a la API de administración: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("la API de administración respondió %d", resp.StatusCode)
	}

	var messages []rabbitMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return fmt.Errorf("decodificando respuesta: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}
	log.Printf("rabbitmq-http-poller: %d mensaje(s) obtenidos de %q", len(messages), p.queue)

	for _, m := range messages {
		var payload []byte
		if m.PayloadEncoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(m.Payload)
			if err != nil {
				log.Printf("rabbitmq-http-poller: no se pudo decodificar payload base64 (routing_key=%q): %v", m.RoutingKey, err)
				continue
			}
			payload = decoded
		} else {
			payload = []byte(m.Payload)
		}

		if err := handler(ctx, payload); err != nil {
			log.Printf("rabbitmq-http-poller: error procesando mensaje (routing_key=%q): %v", m.RoutingKey, err)
		}
	}

	return nil
}