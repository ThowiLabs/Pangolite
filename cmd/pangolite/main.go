package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	case "doctor":
		return doctor(args, stdout)
	case "user":
		return userCommand(args, os.Stdin, stdout)
	case "healthcheck":
		return healthcheck(args)
	case "smoke-backend":
		return smokeBackend(args, stdout)
	case "version":
		fmt.Fprintln(stdout, "pangolite "+app.Version)
		return nil
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	default:
		printHelp(stderr)
		return fmt.Errorf("comando desconocido: %s", cmd)
	}
}

func userCommand(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		printUserHelp(stdout)
		return nil
	}
	switch args[0] {
	case "reset-password", "passwd":
		return resetUserPassword(args[1:], stdin, stdout)
	case "help", "-h", "--help":
		printUserHelp(stdout)
		return nil
	default:
		printUserHelp(stdout)
		return fmt.Errorf("comando de usuario desconocido: %s", args[0])
	}
}

func resetUserPassword(args []string, stdin io.Reader, stdout io.Writer) error {
	cfg := app.LoadConfigFromEnv()
	fs := flag.NewFlagSet("user reset-password", flag.ContinueOnError)
	fs.SetOutput(stdout)
	dataPath := cfg.DataPath
	passwordStdin := false
	requireChange := false
	fs.StringVar(&dataPath, "data", dataPath, "ruta de la base SQLite")
	fs.BoolVar(&passwordStdin, "password-stdin", false, "leer la nueva contraseña desde stdin")
	fs.BoolVar(&requireChange, "require-change", false, "obligar a cambiar la contraseña en el siguiente inicio de sesión")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("uso: pangolite user reset-password [--data RUTA] [--password-stdin] [--require-change] USUARIO")
	}
	username := app.NormalizeUsername(fs.Arg(0))
	if err := app.ValidateUsername(username); err != nil {
		return err
	}
	if strings.TrimSpace(dataPath) == "" {
		return errors.New("ruta de base SQLite requerida")
	}
	info, err := os.Stat(dataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("la base SQLite no existe: %s", dataPath)
		}
		return fmt.Errorf("revisar base SQLite: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("la ruta de base SQLite es un directorio: %s", dataPath)
	}

	store, err := app.NewStore(dataPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := store.UserByUsername(username); err != nil {
		return fmt.Errorf("usuario %q no encontrado", username)
	}

	var password string
	if passwordStdin {
		password, err = readPasswordLine(stdin)
	} else {
		password, err = readPasswordInteractive("Nueva contraseña: ")
		if err == nil {
			var confirmation string
			confirmation, err = readPasswordInteractive("Repite la contraseña: ")
			if err == nil && password != confirmation {
				return errors.New("las contraseñas no coinciden")
			}
		}
	}
	if err != nil {
		return err
	}
	if err := app.ValidatePassword(password); err != nil {
		return err
	}

	user, err := store.ResetUserPassword(username, password, requireChange)
	if err != nil {
		return err
	}
	if err := store.RecordAudit(context.Background(), app.AuditEvent{
		Action:     "user.password.reset_cli",
		EntityType: "user",
		EntityID:   fmt.Sprintf("%d", user.ID),
		Username:   "system",
		Metadata:   `{"source":"cli"}`,
	}); err != nil {
		fmt.Fprintf(stdout, "Advertencia: contraseña restablecida, pero no se pudo registrar la auditoría: %v\n", err)
	}
	fmt.Fprintf(stdout, "Contraseña restablecida para %s. Las sesiones y enlaces de recuperación anteriores fueron invalidados.\n", user.Username)
	return nil
}

func readPasswordLine(r io.Reader) (string, error) {
	reader := bufio.NewReader(io.LimitReader(r, 1024))
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("leer contraseña desde stdin: %w", err)
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	if line == "" && errors.Is(err, io.EOF) {
		return "", errors.New("stdin no contiene una contraseña")
	}
	return line, nil
}

func printUserHelp(w io.Writer) {
	fmt.Fprintln(w, `Uso:
  pangolite user reset-password [--data RUTA] USUARIO
  pangolite user reset-password --password-stdin [--data RUTA] USUARIO

Alias:
  pangolite user passwd ...

Opciones:
  --data            ruta de la base SQLite; por defecto PANGOLITE_DATA
  --password-stdin  lee la contraseña desde stdin para automatización
  --require-change  obliga al usuario a cambiarla tras iniciar sesión`)
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
	logWriter, logWriterErr := app.NewMultiLogWriter(os.Stdout, cfg.LogPath)
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if logWriterErr != nil {
		logger.Warn("no se pudo abrir archivo de logs; se usara solo stdout", "path", cfg.LogPath, "error", logWriterErr.Error())
	}
	logger.Info("configuracion cargada", "config", app.PrintServeConfig(cfg), "initial_password_file", cfg.InitialPasswordFile, "log_file", cfg.LogPath)
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
		effective := store.EffectiveConfig(cfg)
		if err := app.RenderStaticTraefik(effective, store.ListResources()); err != nil {
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
		FallbackURL:  os.Getenv("PANGOLITE_FALLBACK_URL"),
		AgentID:      os.Getenv("PANGOLITE_AGENT_ID"),
		Token:        os.Getenv("PANGOLITE_AGENT_TOKEN"),
		PollInterval: time.Second,
	}
	fs.StringVar(&cfg.ServerURL, "server-url", cfg.ServerURL, "URL publica o interna de Pangolite")
	fs.StringVar(&cfg.FallbackURL, "fallback-url", cfg.FallbackURL, "URL fallback por IP del VPS para redescubrir el panel")
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
	effective := store.EffectiveConfig(cfg)
	if err := app.RenderStaticTraefik(effective, store.ListResources()); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Configuracion de Traefik escrita en %s. HTTP/HTTPS usa recarga dinamica; TCP/UDP nuevos requieren reinicio controlado de Traefik.\n", cfg.TraefikDir)
	return nil
}

func doctor(args []string, stdout io.Writer) error {
	cfg := app.LoadConfigFromEnv()
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stdout)
	app.ApplyCommonFlags(fs, &cfg)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg.ResolveBootstrapPaths()
	return app.RunDoctor(context.Background(), cfg, stdout)
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
  pangolite doctor [flags]
  pangolite user reset-password [--data RUTA] USUARIO
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
