module github.com/osv/converter

go 1.26.3

require (
	github.com/nats-io/nats.go v1.37.0
	github.com/ossf/osv-schema/bindings/go v0.0.0-20260525004216-afe0bddbf893
	github.com/rs/zerolog v1.33.0
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
)

require (
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/osv/pkg => ../pkg
