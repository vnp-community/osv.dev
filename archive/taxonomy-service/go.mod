module github.com/osv/taxonomy-service

go 1.26.3

require (
	github.com/go-chi/chi/v5 v5.2.2
	github.com/redis/go-redis/v9 v9.7.3
	github.com/rs/zerolog v1.33.0
	go.mongodb.org/mongo-driver v1.17.4
	github.com/osv/pkg v0.0.0
)

replace github.com/osv/pkg => ../pkg
