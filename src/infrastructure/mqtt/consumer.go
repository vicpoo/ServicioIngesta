//src/infrastructure/mqtt/consumer.go
// Package mqtt consume mensajes directamente por MQTT desde RabbitMQ (su
// plugin MQTT), como alternativa al consumidor AMQP de
// infrastructure/rabbitmq cuando el puerto 5672 (AMQP) no está expuesto y
// solo el 1883 (MQTT, el mismo que ya usa el ESP32 para publicar) es
// alcanzable.
//
// RabbitMQ traduce automáticamente routing keys de AMQP <-> tópicos MQTT
// reemplazando "." por "/": la routing key "kajve.#" (AMQP, ver
// infrastructure/rabbitmq) equivale al tópico "kajve/#" (MQTT, usado aquí).
package mqtt

import (
	"context"
	"fmt"
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Topic es el equivalente MQTT de la routing key AMQP "kajve.#".
const Topic = "kajve/#"

// MessageHandler procesa el cuerpo crudo de un mensaje. Mismo contrato que
// infrastructure/rabbitmq.MessageHandler.
type MessageHandler func(ctx context.Context, body []byte) error

// Consumer envuelve el cliente MQTT.
type Consumer struct {
	client paho.Client
}

// NewConsumer conecta al broker MQTT y valida credenciales de inmediato
// (fail-fast al arrancar el servicio). Todavía no se suscribe a ningún
// tópico — eso ocurre en Consume, ya con el handler listo, para no perder
// mensajes que puedan llegar justo después de suscribirse.
func NewConsumer(host string, port int, user, pass string) (*Consumer, error) {
	brokerURL := fmt.Sprintf("tcp://%s:%d", host, port)

	opts := paho.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(fmt.Sprintf("ingesta-iot-%d", time.Now().UnixNano()))
	opts.SetUsername(user)
	opts.SetPassword(pass)
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		log.Printf("mqtt: conexión perdida: %v (reconectando automáticamente)", err)
	})

	client := paho.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(15 * time.Second) {
		return nil, fmt.Errorf("mqtt: tiempo de espera agotado conectando a %s", brokerURL)
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt: error conectando a %s: %w", brokerURL, err)
	}

	log.Printf("mqtt: conectado a %s", brokerURL)
	return &Consumer{client: client}, nil
}

// Consume se suscribe a kajve/# y llama a handler por cada mensaje recibido.
// Bloquea hasta que ctx se cancela.
//
// A diferencia del consumidor AMQP (infrastructure/rabbitmq), esta librería
// no expone ack/nack manual tal como está usada aquí (QoS 1, el mensaje se
// da por entregado en cuanto corre el callback). Si handler devuelve error
// (fallo transitorio de BD/Redis), no hay reintento automático a nivel de
// broker — queda registrado en el log. Ver Fase 6 del plan (resiliencia) si
// esto necesita endurecerse más adelante.
func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	onMessage := func(_ paho.Client, msg paho.Message) {
		if err := handler(ctx, msg.Payload()); err != nil {
			log.Printf("mqtt: error procesando mensaje del tópico %q: %v", msg.Topic(), err)
		}
	}

	token := c.client.Subscribe(Topic, 1, onMessage)
	if !token.WaitTimeout(15 * time.Second) {
		return fmt.Errorf("mqtt: tiempo de espera agotado suscribiendo a %q", Topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt: error suscribiendo a %q: %w", Topic, err)
	}
	log.Printf("mqtt: suscrito a %q, esperando mensajes...", Topic)

	<-ctx.Done()
	return ctx.Err()
}

// Close se desconecta del broker MQTT.
func (c *Consumer) Close() error {
	c.client.Disconnect(250)
	return nil
}