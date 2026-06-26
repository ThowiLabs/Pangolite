package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thowilabs/pangolite/internal/app"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	switch cmd {
	case "serve":
		return serve(args, stdout)
	case "agent":
		return agent(args, stdout)
	case "render-traefik":
		return renderTraefik(args, stdout)
	case "healthcheck":
		return healthcheck(args)
	case "smoke-backend":
		return smokeBackend(args, stdout)
	case "version":
		fmt.Fprintln(stdout, "pangolite 0.3.0-system")
		return nil
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	default:
		printHelp(stderr)
		return fmt.Errorf("comando desconocido: %s", cmd)
	}
}

func serve(args []string, stdout io.Writer) error {
	cfg := app.LoadConfigFromEnv()
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stdout)
	app.ApplyCommonFlags(fs, &cfg)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.ResolveBootstrapPaths()
	if err := cfg.ValidateForServe(); err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("configuracion cargada", "config", app.PrintServeConfig(cfg), "initial_password_file", cfg.InitialPasswordFile)
	store, err := app.NewStore(cfg.DataPath)
	if err != nil {
		return err
	}
	defer store.Close()
	created, _, err := store.BootstrapAdmin(cfg.InitialAdminUser, cfg.InitialPasswordFile)
	if err != nil {
		return err
	}
	if created {
		logger.Warn("admin inicial creado; cambia la password temporal", "user", cfg.InitialAdminUser, "password_file", cfg.InitialPasswordFile)
	}
	if cfg.BootstrapTraefik {
		if err := app.RenderStaticTraefik(cfg, store.ListResources()); err != nil {
			return err
		}
		logger.Info("configuracion inicial de Traefik renderizada", "dir", cfg.TraefikDir)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.NewServer(cfg, store, logger).Run(ctx)
}

func agent(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(stdout)
	cfg := app.AgentClientConfig{
		ServerURL:    os.Getenv("PANGOLITE_SERVER_URL"),
		AgentID:      os.Getenv("PANGOLITE_AGENT_ID"),
		Token:        os.Getenv("PANGOLITE_AGENT_TOKEN"),
		PollInterval: time.Second,
	}
	fs.StringVar(&cfg.ServerURL, "server-url", cfg.ServerURL, "URL publica o interna de Pangolite")
	fs.StringVar(&cfg.AgentID, "agent-id", cfg.AgentID, "ID del agente creado en el panel")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "token del agente")
	fs.DurationVar(&cfg.PollInterval, "poll-interval", cfg.PollInterval, "pausa entre polls vacios")
	fs.DurationVar(&cfg.RequestTimeout, "request-timeout", 0, "timeout opcional para requests al backend local; 0 sin timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.RunAgent(ctx, cfg, logger)
}

func renderTraefik(args []string, stdout io.Writer) error {
	cfg := app.LoadConfigFromEnv()
	fs := flag.NewFlagSet("render-traefik", flag.ContinueOnError)
	fs.SetOutput(stdout)
	app.ApplyCommonFlags(fs, &cfg)
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, err := app.NewStore(cfg.DataPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := app.RenderStaticTraefik(cfg, store.ListResources()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Configuracion de Traefik escrita en %s\n", cfg.TraefikDir)
	return nil
}

func healthcheck(args []string) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := fs.String("url", "http://127.0.0.1:2424/healthz", "url de salud")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(*url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("estado no saludable: %s", res.Status)
	}
	return nil
}

func smokeBackend(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("smoke-backend", flag.ContinueOnError)
	fs.SetOutput(stdout)
	addr := fs.String("addr", "127.0.0.1:18081", "direccion del backend HTTP temporal")
	body := fs.String("body", "pangolite-smoke-ok", "respuesta fija del backend")
	if err := fs.Parse(args); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, *body)
	})
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		fmt.Fprintf(stdout, "smoke backend escuchando en %s\n", *addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errc:
		return err
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, `Uso:
  pangolite serve [flags]
  pangolite agent [--server-url https://proxy.example.com --agent-id ID --token TOKEN]
  pangolite render-traefik [flags]
  pangolite healthcheck [--url http://127.0.0.1:2424/healthz]
  pangolite smoke-backend [--addr 127.0.0.1:18081]

Flags comunes:
  --addr                   direccion interna del panel
  --data                   ruta de la base SQLite
  --traefik-dir            directorio de configuracion de Traefik
  --dashboard-domain       dominio del panel
  --email                  correo para Let's Encrypt
  --initial-admin-user     usuario admin inicial
  --initial-password-file  archivo de password temporal inicial

Variables para agente:
  PANGOLITE_SERVER_URL
  PANGOLITE_AGENT_ID
  PANGOLITE_AGENT_TOKEN`)
}
