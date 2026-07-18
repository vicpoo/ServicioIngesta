// Package rabbitmq consume la cola donde el ESP32 publica sus lecturas.
package rabbitmq

import (
	"context"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Topología real ya en uso (dada por el equipo, no inventada):
//   - Exchange: amq.topic (el exchange topic por defecto de RabbitMQ)
//   - Cola:     kajve_datos
//   - Routing key de binding: kajve.#
const (
	ExchangeName = "amq.topic"
	QueueName    = "kajve_datos"
	RoutingKey   = "kajve.#"
	consumerTag  = "ingesta-iot"
)

// MessageHandler procesa el cuerpo crudo de un mensaje. Debe devolver error
// SOLO para fallos transitorios que ameriten reintento (BD/Redis caídos,
// etc.). Un payload inválido o un sensor desconocido se resuelven
// internamente (loguear y descartar, error nil), para no atascar la cola
// reintentando mensajes que nunca van a poder procesarse.
type MessageHandler func(ctx context.Context, body []byte) error

// Consumer envuelve la conexión y el canal AMQP.
type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewConsumer se conecta a RabbitMQ, declara la cola kajve_datos de forma
// idempotente y la enlaza a amq.topic con la routing key kajve.#.
//
// Nota: si la cola ya existe en el broker con argumentos distintos a los
// declarados aquí (durable=true, autoDelete=false, exclusive=false), esta
// llamada puede fallar con PRECONDITION_FAILED. En ese caso, reemplazar
// channel.QueueDeclare por channel.QueueDeclarePassive (que solo verifica
// que la cola exista, sin intentar crearla/redeclararla).
func NewConsumer(amqpURL string) (*Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: error conectando a %q: %w", safeURL(amqpURL), err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: error abriendo canal: %w", err)
	}

	// Prefetch moderado: evita cargar miles de mensajes en memoria a la vez
	// si el ESP32 (u otros dispositivos futuros) publican en ráfaga.
	if err := ch.Qos(20, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: error configurando QoS: %w", err)
	}

	q, err := ch.QueueDeclare(
		QueueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: error declarando la cola %q: %w", QueueName, err)
	}

	if err := ch.QueueBind(q.Name, RoutingKey, ExchangeName, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: error enlazando %q a %q con routing key %q: %w", q.Name, ExchangeName, RoutingKey, err)
	}

	log.Printf("rabbitmq: conectado — cola=%q exchange=%q routing_key=%q", q.Name, ExchangeName, RoutingKey)
	return &Consumer{conn: conn, channel: ch}, nil
}

// Close cierra el canal y la conexión AMQP.
func (c *Consumer) Close() error {
	if err := c.channel.Close(); err != nil {
		return err
	}
	return c.conn.Close()
}

// Consume empieza a recibir mensajes de kajve_datos y llama a handler por
// cada uno, con ack manual. Bloquea hasta que ctx se cancela o el canal de
// RabbitMQ se cierra (por ejemplo, por una caída de conexión).
func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
	msgs, err := c.channel.Consume(
		QueueName,
		consumerTag,
		false, // autoAck: false, ack manual
		false, // exclusive
		false, // noLocal (no usado por RabbitMQ)
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: error registrando el consumidor: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbitmq: el canal de mensajes se cerró (posible caída de conexión)")
			}
			if err := handler(ctx, msg.Body); err != nil {
				log.Printf("rabbitmq: error procesando mensaje, se reintenta: %v", err)
				_ = msg.Nack(false, true) // requeue=true: fallo transitorio
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

// safeURL evita loguear la contraseña si algún día se imprime el error de
// conexión completo.
func safeURL(_ string) string {
	return "<amqp url oculta>"
}
