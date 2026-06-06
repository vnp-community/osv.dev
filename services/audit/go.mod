module github.com/defectdojo/audit

go 1.22

require (
	github.com/defectdojo/pkg v0.0.0
	github.com/go-chi/chi/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/nats-io/nats.go v1.37.0
	github.com/rs/zerolog v1.33.0
)

replace github.com/defectdojo/pkg => ../dd-pkg
