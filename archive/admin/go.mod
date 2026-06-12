module github.com/osv/admin

go 1.26.3

require (
	github.com/osv/pkg v0.0.0
	github.com/ossf/osv-schema/bindings/go v0.0.0-20260525004216-afe0bddbf893
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af
)

replace github.com/osv/pkg => ../pkg
