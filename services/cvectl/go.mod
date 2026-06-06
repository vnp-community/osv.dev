module github.com/osv/cvectl

go 1.22

require (
	github.com/osv/pkg v0.0.0
	github.com/rs/zerolog v1.33.0
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/osv/pkg => ../pkg
