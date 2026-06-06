module github.com/globalcve/kev-service

go 1.26.3

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/jackc/pgx/v5 v5.10.0
	github.com/osv/pkg v0.0.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/osv/pkg => ../pkg
