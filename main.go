//main.go
// Servicio de Ingesta IoT — kajve.
//
// Consume vía MQTT (tópico kajve/#, equivalente a la routing key AMQP
// kajve.# que usa la cola kajve_datos) los mensajes que el ESP32 publica,
// resuelve a qué productor y lote pertenece cada dato, lo persiste en
// Postgres respetando RLS, y lo publica en tiempo real por Redis Pub/Sub
// para que el WebSocket Gateway lo entregue únicamente al dueño del osil.
//
// Además, si RABBITMQ_MGMT_URL está configurado, drena en paralelo la cola
// kajve_datos vía la API HTTP de administración de RabbitMQ — es un
// respaldo mientras el puerto AMQP 5672 no esté expuesto en el servidor.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/kajve/ingesta-iot/src/infrastructure/dependencies"
	"github.com/kajve/ingesta-iot/src/infrastructure/routes"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("main: no se encontró archivo .env, se usan las variables de entorno del sistema")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	container, err := dependencies.NewContainer(ctx)
	if err != nil {
		log.Fatalf("main: error inicializando dependencias: %v", err)
	}
	defer container.Close()

	// Consumidor MQTT en segundo plano: es el corazón del servicio.
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		log.Println("main: iniciando consumidor MQTT")
		if err := container.Consumer.Consume(ctx, container.IngestaService.HandleMessage); err != nil {
			log.Printf("main: el consumidor MQTT terminó: %v", err)
		}
	}()

	// Poller HTTP de RabbitMQ (opcional): drena kajve_datos directamente,
	// incluyendo el backlog que ya estaba acumulado antes de que este
	// servicio arrancara.
	if container.HTTPPoller != nil {
		go func() {
			log.Println("main: iniciando drenado HTTP de RabbitMQ")
			container.HTTPPoller.Run(ctx, container.IngestaService.HandleMessage)
		}()
	}

	// Servidor HTTP en :8001, principalmente para health checks.
	router := routes.NewRouter()
	server := &http.Server{
		Addr:              ":" + container.Config.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("main: servidor HTTP escuchando en :%s", container.Config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("main: error en el servidor HTTP: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("main: señal de apagado recibida, cerrando...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("main: error cerrando el servidor HTTP: %v", err)
	}

	<-consumerDone
	log.Println("main: apagado completo")
}