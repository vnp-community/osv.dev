// Command globalcve-mono is the GlobalCVE monolithic application.
//
// It runs all services (CVE Search, CVE Sync, KEV, Notification, API Gateway)
// as independent goroutines within a single process, communicating via
// direct function calls, HTTP (internal), and NATS JetStream events.
//
// Usage:
//
//	globalcve-mono [--config path/to/config.yaml]
//	globalcve-mono migrate [--config path/to/config.yaml]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/globalcve/mono/internal/app"
	"github.com/globalcve/mono/internal/config"
)

var (
	configFile = flag.String("config", "config/config.yaml", "Path to config file")
)

func main() {
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	application := app.New(cfg)
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}
