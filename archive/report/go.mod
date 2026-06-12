module github.com/defectdojo/report

go 1.22

require (
	github.com/defectdojo/pkg v0.0.0
	github.com/defectdojo/proto v0.0.0
	github.com/go-chi/chi/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.37.0
	github.com/rs/zerolog v1.33.0
	github.com/xuri/excelize/v2 v2.9.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
)

replace (
	github.com/defectdojo/pkg => ../dd-pkg
	github.com/defectdojo/proto => ../dd-proto
)
