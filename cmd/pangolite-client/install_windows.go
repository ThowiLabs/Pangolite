//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/thowilabs/pangolite/internal/app"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	clientServiceName = "PangoliteClient"
	clientInstallDir  = `C:\ProgramData\Pangolite Client`
	clientBinPath     = `C:\ProgramData\Pangolite Client\pangolite-client.exe`
	clientEnvPath     = `C:\ProgramData\Pangolite Client\pangolite-client.env`

	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	swShowNormal          = 1
)

var shellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.Handle
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     unsafe.Pointer
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

type pangoliteService struct{}

func ensureClientPrivileges(args []string, needed bool, stdout io.Writer) (bool, error) {
	if !needed || windows.GetCurrentProcessToken().IsElevated() {
		return false, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return false, err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return false, err
	}
	params, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(args))
	if err != nil {
		return false, err
	}
	cwd, _ := os.Getwd()
	var cwdPtr *uint16
	if cwd != "" {
		cwdPtr, _ = windows.UTF16PtrFromString(cwd)
	}
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  cwdPtr,
		nShow:        swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	fmt.Fprintln(stdout, "Se requieren privilegios de administrador; solicitando elevacion de Windows...")
	r1, _, callErr := shellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return false, fmt.Errorf("solicitar elevacion UAC: %w", callErr)
	}
	if info.hProcess == 0 {
		return false, errors.New("Windows no devolvio el proceso elevado")
	}
	defer windows.CloseHandle(info.hProcess)
	if _, err := windows.WaitForSingleObject(info.hProcess, windows.INFINITE); err != nil {
		return false, fmt.Errorf("esperar proceso elevado: %w", err)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err != nil {
		return false, fmt.Errorf("leer resultado del proceso elevado: %w", err)
	}
	if exitCode != 0 {
		return false, fmt.Errorf("la operacion elevada termino con codigo %d", exitCode)
	}
	return true, nil
}

func installClient(stdout io.Writer, cfg app.AgentClientConfig) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	hadFiles := pathExists(clientInstallDir)
	hadService, err := removeWindowsService()
	if err != nil {
		return err
	}
	if hadFiles || hadService {
		fmt.Fprintln(stdout, "Instalacion anterior detectada; reemplazando pangolite-client...")
	}
	if err := os.MkdirAll(clientInstallDir, 0o700); err != nil {
		return err
	}
	if !sameWindowsPath(exe, clientBinPath) {
		if err := os.Remove(clientBinPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("retirar binario anterior: %w", err)
		}
		if err := copyFile(exe, clientBinPath, 0o755); err != nil {
			return err
		}
	}
	env := fmt.Sprintf("PANGOLITE_SERVER_URL=%s\nPANGOLITE_FALLBACK_URL=%s\nPANGOLITE_AGENT_ID=%s\nPANGOLITE_AGENT_TOKEN=%s\n", cfg.ServerURL, cfg.FallbackURL, cfg.AgentID, cfg.Token)
	if err := os.WriteFile(clientEnvPath, []byte(env), 0o600); err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("conectar al administrador de servicios: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.CreateService(clientServiceName, clientBinPath, mgr.Config{
		DisplayName: "Pangolite Client",
		Description: "Cliente de sistema Pangolite",
		StartType:   mgr.StartAutomatic,
	}, "--service")
	if err != nil {
		return fmt.Errorf("crear servicio Windows: %w", err)
	}
	defer service.Close()
	recovery := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 15 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}
	if err := service.SetRecoveryActions(recovery, 300); err != nil {
		return fmt.Errorf("configurar recuperacion del servicio Windows: %w", err)
	}
	if err := service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return fmt.Errorf("configurar recuperacion por fallo del servicio Windows: %w", err)
	}
	if err := service.Start(); err != nil {
		return fmt.Errorf("iniciar servicio Windows: %w", err)
	}
	fmt.Fprintln(stdout, "Cliente instalado y arrancado como servicio de Windows")
	return nil
}

func removeWindowsService() (bool, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return false, fmt.Errorf("conectar al administrador de servicios: %w", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(clientServiceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return false, nil
		}
		return false, fmt.Errorf("abrir servicio Windows: %w", err)
	}
	status, queryErr := service.Query()
	if queryErr == nil && status.State != svc.Stopped {
		if status.State != svc.StopPending {
			if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
				_ = service.Close()
				return true, fmt.Errorf("detener servicio Windows: %w", err)
			}
		}
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			status, err = service.Query()
			if err != nil {
				break
			}
			if status.State == svc.Stopped {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		if err == nil && status.State != svc.Stopped {
			_ = service.Close()
			return true, errors.New("el servicio Windows no se detuvo dentro de 15 segundos")
		}
	}
	if err := service.Delete(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		_ = service.Close()
		return true, fmt.Errorf("eliminar servicio Windows anterior: %w", err)
	}
	if err := service.Close(); err != nil {
		return true, err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		probe, err := manager.OpenService(clientServiceName)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return true, nil
		}
		if err != nil {
			return true, fmt.Errorf("confirmar eliminacion del servicio Windows: %w", err)
		}
		_ = probe.Close()
		time.Sleep(200 * time.Millisecond)
	}
	return true, errors.New("el servicio Windows anterior sigue marcado para eliminacion")
}

func removeClient(stdout io.Writer) error {
	if _, err := removeWindowsService(); err != nil {
		return err
	}
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
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func sameWindowsPath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func shellValue(v string) string { return v }

func defaultClientEnvPath() string { return clientEnvPath }
