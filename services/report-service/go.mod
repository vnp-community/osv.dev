module github.com/osv/report-service

go 1.26.3

replace github.com/osv/pkg => ../pkg

replace github.com/osv/proto => ../proto

require (
	github.com/jung-kurt/gofpdf v1.16.2
	github.com/osv/proto v0.0.0
	github.com/rs/zerolog v1.35.1
	google.golang.org/grpc v1.71.0
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)
