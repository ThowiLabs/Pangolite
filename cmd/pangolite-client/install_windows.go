//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thowilabs/pangolite/internal/app"
	"golang.org/x/sys/windows/svc"
)

const (
	clientServiceName = "PangoliteClient"
	clientInstallDir  = `C:\ProgramData\Pangolite Client`
	clientBinPath     = `C:\ProgramData\Pangolite Client\pangolite-client.exe`
	clientEnvPath     = `C:\ProgramData\Pangolite Client\pangolite-client.env`
)

type pangoliteService struct{}

func installClient(stdout io.Writer, cfg app.AgentClientConfig) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(clientInstallDir, 0o700); err != nil {
		return err
	}
	if err := copyFile(exe, clientBinPath, 0o755); err != nil {
		return err
	}
	env := fmt.Sprintf("PANGOLITE_SERVER_URL=%s\nPANGOLITE_FALLBACK_URL=%s\nPANGOLITE_AGENT_ID=%s\nPANGOLITE_AGENT_TOKEN=%s\n", cfg.ServerURL, cfg.FallbackURL, cfg.AgentID, cfg.Token)
	if err := os.WriteFile(clientEnvPath, []byte(env), 0o600); err != nil {
		return err
	}
	_ = exec.Command("sc.exe", "stop", clientServiceName).Run()
	_ = exec.Command("sc.exe", "delete", clientServiceName).Run()
	binPath := fmt.Sprintf(`"%s" --service`, clientBinPath)
	if out, err := exec.Command("sc.exe", "create", clientServiceName, "binPath=", binPath, "start=", "auto", "DisplayName=", "Pangolite Client").CombinedOutput(); err != nil {
		return fmt.Errorf("crear servicio Windows: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("sc.exe", "start", clientServiceName).CombinedOutput(); err != nil {
		return fmt.Errorf("iniciar servicio Windows: %s", strings.TrimSpace(string(out)))
	}
	fmt.Fprintln(stdout, "Cliente instalado y arrancado como servicio de Windows")
	return nil
}

func removeClient(stdout io.Writer) error {
	_ = exec.Command("sc.exe", "stop", clientServiceName).Run()
	_ = exec.Command("sc.exe", "delete", clientServiceName).Run()
	if err := os.RemoveAll(clientInstallDir); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Cliente eliminado de Windows")
	return nil
}

func runService(stdout io.Writer) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		cfg, err := loadWindowsConfig()
		if err != nil {
			return err
		}
		return runForeground(cfg)
	}
	return svc.Run(clientServiceName, pangoliteService{})
}

func (pangoliteService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	cfg, err := loadWindowsConfig()
	if err != nil {
		return true, 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.RunAgent(ctx, cfg, logger)
	}()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range req {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			cancel()
			select {
			case <-done:
			case <-time.After(15 * time.Second):
			}
			return false, 0
		default:
		}
	}
	cancel()
	return false, 0
}

func loadWindowsConfig() (app.AgentClientConfig, error) {
	b, err := os.ReadFile(clientEnvPath)
	if err != nil {
		return app.AgentClientConfig{}, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	cfg := app.AgentClientConfig{ServerURL: m["PANGOLITE_SERVER_URL"], FallbackURL: m["PANGOLITE_FALLBACK_URL"], ConfigPath: clientEnvPath, AgentID: m["PANGOLITE_AGENT_ID"], Token: m["PANGOLITE_AGENT_TOKEN"], PollInterval: time.Second}
	if err := cfg.Validate(); err != nil {
		return app.AgentClientConfig{}, err
	}
	return cfg, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func shellValue(v string) string { return v }

func defaultClientEnvPath() string { return clientEnvPath }
