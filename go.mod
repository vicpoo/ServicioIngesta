module github.com/kajve/ingesta-iot

go 1.24.0

require (
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/go-chi/chi/v5 v5.1.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/joho/godotenv v1.5.1
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/redis/go-redis/v9 v9.6.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace (
	golang.org/x/crypto => github.com/golang/crypto v0.17.0
	golang.org/x/sync => github.com/golang/sync v0.1.0
	golang.org/x/text => github.com/golang/text v0.14.0
	gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20200902074654-038fdea0a05b
	gopkg.in/yaml.v3 => github.com/go-yaml/yaml/v3 v3.0.1
)
