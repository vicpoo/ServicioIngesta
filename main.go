// Servicio de Ingesta IoT — kajve.
//
// Consume la cola kajve_datos (exchange amq.topic, routing key kajve.#)
// donde el ESP32 publica sus lecturas, resuelve a qué productor y lote
// pertenece cada dato, lo persiste en Postgres respetando RLS, y lo publica
// en tiempo real por Redis Pub/Sub para que el WebSocket Gateway lo
// entregue únicamente al dueño del osil.
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

	// Consumidor de RabbitMQ en segundo plano: es el corazón del servicio.
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		log.Printf("main: iniciando consumidor de RabbitMQ (cola=%q)", "kajve_datos")
		if err := container.Consumer.Consume(ctx, container.IngestaService.HandleMessage); err != nil {
			log.Printf("main: el consumidor de RabbitMQ terminó: %v", err)
		}
	}()

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
