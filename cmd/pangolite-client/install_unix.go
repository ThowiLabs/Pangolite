//go:build !windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thowilabs/pangolite/internal/app"
)

const (
	clientInstallDir = "/opt/pangolite-client"
	clientBinPath    = "/opt/pangolite-client/pangolite-client"
	clientEnvPath    = "/opt/pangolite-client/pangolite-client.env"
)

func installClient(stdout io.Writer, cfg app.AgentClientConfig) error {
	if os.Geteuid() != 0 {
		return errors.New("ejecuta --install como root")
	}
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
	env := fmt.Sprintf("PANGOLITE_SERVER_URL=%s\nPANGOLITE_AGENT_ID=%s\nPANGOLITE_AGENT_TOKEN=%s\n", shellValue(cfg.ServerURL), shellValue(cfg.AgentID), shellValue(cfg.Token))
	if err := os.WriteFile(clientEnvPath, []byte(env), 0o600); err != nil {
		return err
	}
	if hasCommand("systemctl") && isSystemd() {
		if err := installSystemd(); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Cliente instalado y arrancado con systemd")
		return nil
	}
	if hasCommand("rc-service") && hasCommand("rc-update") {
		if err := installOpenRC(); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "Cliente instalado y arrancado con OpenRC")
		return nil
	}
	return errors.New("no se detecto systemd ni OpenRC; ejecuta el binario manualmente o crea un servicio de tu init system")
}

func removeClient(stdout io.Writer) error {
	if os.Geteuid() != 0 {
		return errors.New("ejecuta --remove como root")
	}
	if hasCommand("systemctl") {
		_ = exec.Command("systemctl", "disable", "--now", "pangolite-client").Run()
		_ = os.Remove("/etc/systemd/system/pangolite-client.service")
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "reset-failed", "pangolite-client").Run()
	}
	if hasCommand("rc-service") {
		_ = exec.Command("rc-service", "pangolite-client", "stop").Run()
	}
	if hasCommand("rc-update") {
		_ = exec.Command("rc-update", "del", "pangolite-client", "default").Run()
	}
	_ = os.Remove("/etc/init.d/pangolite-client")
	if err := os.RemoveAll(clientInstallDir); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Cliente eliminado del sistema")
	return nil
}

func runService(stdout io.Writer) error {
	fmt.Fprintln(stdout, "--service solo esta disponible en Windows; en Linux usa systemd/OpenRC")
	return runForeground(app.AgentClientConfig{ServerURL: os.Getenv("PANGOLITE_SERVER_URL"), AgentID: os.Getenv("PANGOLITE_AGENT_ID"), Token: os.Getenv("PANGOLITE_AGENT_TOKEN")})
}

func installSystemd() error {
	service := `[Unit]
Description=Pangolite NAT client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/opt/pangolite-client/pangolite-client.env
ExecStart=/opt/pangolite-client/pangolite-client
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
UMask=0077
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/pangolite-client.service", []byte(service), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "enable", "--now", "pangolite-client").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable --now: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func installOpenRC() error {
	script := `#!/sbin/openrc-run
name="pangolite-client"
description="Pangolite NAT client"
command="/opt/pangolite-client/pangolite-client"
command_background="yes"
pidfile="/run/pangolite-client.pid"
output_log="/var/log/pangolite-client.log"
error_log="/var/log/pangolite-client.err"

depend() {
    need net
    after firewall
}

start_pre() {
    checkpath --directory --mode 0700 /opt/pangolite-client
    set -a
    . /opt/pangolite-client/pangolite-client.env
    set +a
}
`
	if err := os.WriteFile("/etc/init.d/pangolite-client", []byte(script), 0o755); err != nil {
		return err
	}
	if out, err := exec.Command("rc-update", "add", "pangolite-client", "default").CombinedOutput(); err != nil {
		return fmt.Errorf("rc-update add: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("rc-service", "pangolite-client", "restart").CombinedOutput(); err != nil {
		return fmt.Errorf("rc-service restart: %s", strings.TrimSpace(string(out)))
	}
	return nil
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

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err == nil {
		return true
	}
	return false
}

func shellValue(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'"
}
