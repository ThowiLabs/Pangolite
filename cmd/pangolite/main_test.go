package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thowilabs/pangolite/internal/app"
)

func TestUserResetPasswordFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := app.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	created, oldPassword, err := store.BootstrapAdmin("admin", "")
	if err != nil || !created {
		store.Close()
		t.Fatalf("bootstrap invalido: created=%v err=%v", created, err)
	}
	store.Close()

	var out bytes.Buffer
	if err := userCommand([]string{"reset-password", "--data", path, "--password-stdin", "admin"}, strings.NewReader("clave-cli-segura\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Contraseña restablecida para admin") {
		t.Fatalf("salida inesperada: %q", out.String())
	}
	if strings.Contains(out.String(), "clave-cli-segura") {
		t.Fatal("la salida no debe exponer la contraseña")
	}

	store, err = app.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, ok := store.AuthenticateUser("admin", oldPassword); ok {
		t.Fatal("la contraseña anterior no debe autenticar")
	}
	if _, ok := store.AuthenticateUser("admin", "clave-cli-segura"); !ok {
		t.Fatal("la contraseña CLI debe autenticar")
	}
	events, err := store.ListAuditEvents(20, "")
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, event := range events {
		if event.Action == "user.password.reset_cli" && event.EntityType == "user" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatal("se esperaba auditoría del reset CLI")
	}
}

func TestReadPasswordLinePreservesSpaces(t *testing.T) {
	password, err := readPasswordLine(strings.NewReader(" clave con espacios \r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if password != " clave con espacios " {
		t.Fatalf("contraseña alterada: %q", password)
	}
}

func TestUserResetPasswordDoesNotCreateMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inexistente", "pangolite.db")
	var out bytes.Buffer
	err := userCommand([]string{"reset-password", "--data", path, "--password-stdin", "admin"}, strings.NewReader("clave-cli-segura\n"), &out)
	if err == nil || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("se esperaba error de base inexistente, recibido: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("el comando no debe crear una base nueva: %v", statErr)
	}
}
