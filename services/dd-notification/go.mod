module github.com/defectdojo/notification

go 1.22

require (
	github.com/defectdojo/pkg v0.0.0
	github.com/go-chi/chi/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/nats-io/nats.go v1.37.0
	github.com/rs/zerolog v1.33.0
	github.com/slack-go/slack v0.15.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

replace github.com/defectdojo/pkg => ../dd-pkg
