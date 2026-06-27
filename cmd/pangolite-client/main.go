package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/thowilabs/pangolite/internal/app"
)

const clientName = "pangolite-client"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	var install bool
	var remove bool
	var service bool
	cfg := app.AgentClientConfig{
		ServerURL:    os.Getenv("PANGOLITE_SERVER_URL"),
		AgentID:      os.Getenv("PANGOLITE_AGENT_ID"),
		Token:        os.Getenv("PANGOLITE_AGENT_TOKEN"),
		PollInterval: time.Second,
	}
	fs := flag.NewFlagSet(clientName, flag.ContinueOnError)
	fs.SetOutput(stdout)
	fs.BoolVar(&install, "install", false, "instala el cliente como servicio del sistema")
	fs.BoolVar(&remove, "remove", false, "detiene y elimina el cliente del sistema")
	fs.BoolVar(&service, "service", false, "ejecuta el cliente bajo el administrador de servicios del sistema")
	fs.StringVar(&cfg.ServerURL, "server-url", cfg.ServerURL, "URL publica de Pangolite")
	fs.StringVar(&cfg.AgentID, "agent-id", cfg.AgentID, "ID del cliente creado en el panel")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "token del cliente")
	fs.DurationVar(&cfg.PollInterval, "poll-interval", cfg.PollInterval, "pausa entre polls vacios")
	fs.DurationVar(&cfg.RequestTimeout, "request-timeout", 0, "timeout opcional para requests HTTP al servicio local")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if service {
		return runService(stdout)
	}
	if remove {
		return removeClient(stdout)
	}
	if install {
		if err := cfg.Validate(); err != nil {
			return err
		}
		return installClient(stdout, cfg)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stdout, `Uso:
  pangolite-client --install --server-url https://panel.example.com --agent-id ID --token TOKEN
  pangolite-client --remove
  pangolite-client --server-url https://panel.example.com --agent-id ID --token TOKEN`)
		return err
	}
	return runForeground(cfg)
}

func runForeground(cfg app.AgentClientConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := app.RunAgent(ctx, cfg, logger)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
