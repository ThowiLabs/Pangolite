package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ServiceManagerKind string

const (
	ServiceManagerUnknown ServiceManagerKind = "unknown"
	ServiceManagerSystemd ServiceManagerKind = "systemd"
	ServiceManagerOpenRC  ServiceManagerKind = "openrc"
	ServiceManagerSysV    ServiceManagerKind = "sysvinit"
	ServiceManagerRunit   ServiceManagerKind = "runit"
)

type ServiceManager struct {
	Kind ServiceManagerKind
}

func DetectServiceManager() ServiceManager {
	if _, err := exec.LookPath("systemctl"); err == nil {
		if _, statErr := os.Stat("/run/systemd/system"); statErr == nil {
			return ServiceManager{Kind: ServiceManagerSystemd}
		}
	}
	if _, err := exec.LookPath("rc-service"); err == nil {
		return ServiceManager{Kind: ServiceManagerOpenRC}
	}
	if _, err := exec.LookPath("sv"); err == nil {
		return ServiceManager{Kind: ServiceManagerRunit}
	}
	if _, err := exec.LookPath("service"); err == nil {
		return ServiceManager{Kind: ServiceManagerSysV}
	}
	return ServiceManager{Kind: ServiceManagerUnknown}
}

func (m ServiceManager) Available() bool {
	return m.Kind != ServiceManagerUnknown
}

func (m ServiceManager) String() string {
	if m.Kind == "" {
		return string(ServiceManagerUnknown)
	}
	return string(m.Kind)
}

func RestartService(ctx context.Context, name string) (string, error) {
	manager := DetectServiceManager()
	if !manager.Available() {
		return manager.String(), fmt.Errorf("no se detecto gestor de servicios compatible para reiniciar %s", name)
	}
	return manager.String(), manager.Restart(ctx, name)
}

func (m ServiceManager) Restart(ctx context.Context, name string) error {
	var cmd *exec.Cmd
	switch m.Kind {
	case ServiceManagerSystemd:
		cmd = exec.CommandContext(ctx, "systemctl", "restart", name)
	case ServiceManagerOpenRC:
		cmd = exec.CommandContext(ctx, "rc-service", name, "restart")
	case ServiceManagerSysV:
		cmd = exec.CommandContext(ctx, "service", name, "restart")
	case ServiceManagerRunit:
		cmd = exec.CommandContext(ctx, "sv", "restart", name)
	default:
		return fmt.Errorf("gestor de servicios no soportado: %s", m.String())
	}
	out, err := cmd.CombinedOutput()
	if err != nil && m.Kind == ServiceManagerSysV {
		if _, statErr := os.Stat("/etc/init.d/" + name); statErr == nil {
			cmd = exec.CommandContext(ctx, "/etc/init.d/"+name, "restart")
			out, err = cmd.CombinedOutput()
		}
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s restart %s: %s", m.String(), name, msg)
	}
	return nil
}

func ServiceState(name string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	manager := DetectServiceManager()
	if !manager.Available() {
		return "", false
	}
	return manager.State(ctx, name)
}

func (m ServiceManager) State(ctx context.Context, name string) (string, bool) {
	var cmd *exec.Cmd
	switch m.Kind {
	case ServiceManagerSystemd:
		cmd = exec.CommandContext(ctx, "systemctl", "is-active", name)
	case ServiceManagerOpenRC:
		cmd = exec.CommandContext(ctx, "rc-service", name, "status")
	case ServiceManagerSysV:
		cmd = exec.CommandContext(ctx, "service", name, "status")
	case ServiceManagerRunit:
		cmd = exec.CommandContext(ctx, "sv", "status", name)
	default:
		return "", false
	}
	out, err := cmd.CombinedOutput()
	state := strings.TrimSpace(string(out))
	if state == "" && err != nil {
		state = err.Error()
	}
	if state == "" {
		state = "sin salida"
	}
	return fmt.Sprintf("%s: %s", m.String(), state), true
}
