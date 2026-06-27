package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func RunDoctor(ctx context.Context, c Config, w io.Writer) error {
	checks := []DoctorCheck{}
	add := func(status, name, message string) {
		checks = append(checks, DoctorCheck{Name: name, Status: status, Message: message})
	}

	add("ok", "version", Version)
	manager := DetectServiceManager()
	if manager.Available() {
		add("ok", "gestor servicios", manager.String())
	} else {
		add("warn", "gestor servicios", "no detectado")
	}
	if c.DataPath == "" {
		add("fail", "sqlite", "PANGOLITE_DATA no configurado")
	} else if _, err := os.Stat(c.DataPath); err != nil {
		add("warn", "sqlite", "base aun no existe: "+c.DataPath)
	} else {
		store, err := NewStore(c.DataPath)
		if err != nil {
			add("fail", "sqlite", err.Error())
		} else {
			version, err := store.SchemaVersion(ctx)
			if err != nil {
				add("warn", "migraciones", err.Error())
			} else {
				add("ok", "migraciones", fmt.Sprintf("schema_version=%d", version))
			}
			projects := len(store.ListProjects())
			resources := len(store.ListResources())
			agents := len(store.ListAgents())
			add("ok", "datos", fmt.Sprintf("proyectos=%d recursos=%d clientes_sistema=%d", projects, resources, agents))
			_ = store.Close()
		}
	}

	checkWritableDir := func(name, dir string) {
		if dir == "" {
			add("fail", name, "ruta vacia")
			return
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			add("fail", name, err.Error())
			return
		}
		test := filepath.Join(dir, ".pangolite-doctor")
		if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
			add("fail", name, "sin escritura: "+err.Error())
			return
		}
		_ = os.Remove(test)
		add("ok", name, dir)
	}
	checkWritableDir("backups", c.BackupDir)
	checkWritableDir("plantillas", c.SuspensionTemplateDir)

	if path, err := exec.LookPath("traefik"); err != nil {
		add("warn", "traefik", "binario no encontrado en PATH")
	} else {
		add("ok", "traefik", path)
		if err := ValidateTraefikConfig(c); err != nil {
			add("warn", "traefik-check", err.Error())
		} else {
			add("ok", "traefik-check", "configuracion valida o validacion omitida")
		}
	}
	if c.TraefikDir != "" {
		for _, file := range []string{"traefik.yml", "acme.json"} {
			path := filepath.Join(c.TraefikDir, file)
			if _, err := os.Stat(path); err != nil {
				add("warn", file, "no existe: "+path)
			} else {
				add("ok", file, path)
			}
		}
	}

	checkTCPPort := func(port string) {
		ln, err := net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			add("ok", "puerto "+port, "ocupado/escuchando: "+err.Error())
			return
		}
		_ = ln.Close()
		add("warn", "puerto "+port, "libre; Traefik podria no estar escuchando")
	}
	checkTCPPort("80")
	checkTCPPort("443")

	if serviceState, ok := ServiceState("pangolite"); ok {
		add("ok", "servicio pangolite", serviceState)
	} else {
		add("warn", "servicio pangolite", "no se pudo consultar estado")
	}
	if serviceState, ok := ServiceState("traefik"); ok {
		add("ok", "servicio traefik", serviceState)
	} else {
		add("warn", "servicio traefik", "no se pudo consultar estado")
	}

	failures := 0
	for _, check := range checks {
		if check.Status == "fail" {
			failures++
		}
		fmt.Fprintf(w, "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
	if failures > 0 {
		return fmt.Errorf("doctor encontro %d problema(s) critico(s)", failures)
	}
	return nil
}
