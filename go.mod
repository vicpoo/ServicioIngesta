module github.com/kajve/ingesta-iot

go 1.22

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/joho/godotenv v1.5.1
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/redis/go-redis/v9 v9.6.1
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

// Redirecciones a los mirrors oficiales en GitHub de estas dependencias
// transitivas (mantenidos por el equipo de Go / la comunidad). No cambian
// ninguna versión ni comportamiento; solo evitan depender de la resolución
// "go-get" de sus vanity import paths (golang.org/x/..., gopkg.in/...),
// necesaria en redes con acceso restringido (proxies corporativos, CI con
// egress limitado, etc.). Se pueden quitar sin problema si no aplica.
replace (
	golang.org/x/crypto => github.com/golang/crypto v0.17.0
	golang.org/x/sync => github.com/golang/sync v0.1.0
	golang.org/x/text => github.com/golang/text v0.14.0
	gopkg.in/check.v1 => github.com/go-check/check v0.0.0-20200902074654-038fdea0a05b
	gopkg.in/yaml.v3 => github.com/go-yaml/yaml v3.0.1+incompatible
)
