module github.com/osv/scanner

go 1.26.3

require (
	github.com/klauspost/compress v1.18.5
	github.com/osv/proto v0.0.0-00010101000000-000000000000
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.81.1
)

require (
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ulikunitz/xz v0.5.12
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/osv/pkg => ../pkg

replace github.com/osv/proto => ../proto
