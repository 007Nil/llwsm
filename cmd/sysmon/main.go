package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"llwsm/internal/api"
	"llwsm/internal/config"
	"llwsm/internal/host"
	"llwsm/internal/version"
)

func main() {
	var (
		configPath  = flag.String("config", "", "path to a YAML config file")
		listenAddr  = flag.String("listen", "", "listen address as host:port (overrides config)")
		rootDir     = flag.String("root", "", "host filesystem root; use /host when monitoring a host from a container")
		showVersion = flag.Bool("version", false, "print version information and exit")
	)
	flag.Parse()

	if *showVersion {
		v := version.Get()
		fmt.Printf("sysmon %s (commit %s, built %s)\n", v.Version, v.Commit, v.BuildDate)
		return
	}

	if *configPath != "" {
		if _, err := os.Stat(*configPath); err != nil {
			log.Printf("warning: config file %s not found, using defaults", *configPath)
		}
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *listenAddr != "" {
		hostPart, port, err := net.SplitHostPort(*listenAddr)
		if err != nil {
			log.Fatalf("--listen must be host:port, got %q", *listenAddr)
		}
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum <= 0 || portNum > 65535 {
			log.Fatalf("invalid port in --listen: %q", *listenAddr)
		}
		cfg.Server.Host = hostPart
		cfg.Server.Port = portNum
	}
	if *rootDir != "" {
		cfg.Root = *rootDir
	}

	h := host.New(cfg)
	handler := api.New(h, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go refreshLoop(ctx, h, handler, cfg.Interval())

	ln, err := net.Listen("tcp", cfg.ListenAddr())
	if err != nil {
		log.Fatalf("listen %s: %v", cfg.ListenAddr(), err)
	}
	if !isLoopback(cfg.Server.Host) {
		log.Printf("WARNING: listening on all interfaces (%s). No authentication is implemented; only expose sysmon on a trusted network.", cfg.ListenAddr())
	}
	log.Printf("sysmon %s listening on %s", version.Version, ln.Addr().String())

	srv := &http.Server{
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
	log.Printf("stopped")
}

func refreshLoop(ctx context.Context, h *host.Host, handler *api.Handler, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.Refresh()
			handler.Publish()
		}
	}
}

func loadConfig(flagPath string) (*config.Config, error) {
	path := flagPath
	if path == "" {
		path = os.Getenv("SYSMON_CONFIG")
	}
	if path == "" {
		for _, candidate := range []string{"/etc/sysmon/config.yaml", "config.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	return config.Load(path)
}

func isLoopback(host string) bool {
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
