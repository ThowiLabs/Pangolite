package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBulkSetResourcesEnabledIsProjectScoped(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "pangolite.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	p1, _ := store.AddProject(Project{Name: "Uno"})
	p2, _ := store.AddProject(Project{Name: "Dos"})
	r1, err := store.AddResource(Resource{ProjectID: p1.ID, Name: "A", Mode: ModeTCP, PublicPort: 2201, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := store.AddResource(Resource{ProjectID: p1.ID, Name: "B", Mode: ModeTCP, PublicPort: 2202, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rOther, err := store.AddResource(Resource{ProjectID: p2.ID, Name: "C", Mode: ModeTCP, PublicPort: 2203, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.BulkSetResourcesEnabled(context.Background(), p1.ID, []string{r1.ID, r2.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 2 || updated[0].Enabled || updated[1].Enabled {
		t.Fatalf("resultado inesperado: %+v", updated)
	}
	if _, err := store.BulkSetResourcesEnabled(context.Background(), p1.ID, []string{r1.ID, rOther.ID}, true); err == nil {
		t.Fatal("se esperaba rechazar recurso de otro proyecto")
	}
	other, _ := store.ResourceByID(rOther.ID)
	if !other.Enabled {
		t.Fatal("el recurso ajeno no debe modificarse")
	}
}

func TestNormalizeBulkIDs(t *testing.T) {
	ids, err := normalizeBulkIDs([]string{"abc12345", "abc12345", "def67890"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("se esperaban 2 ids, got %d", len(ids))
	}
	if _, err := normalizeBulkIDs(nil); err == nil {
		t.Fatal("se esperaba error para seleccion vacia")
	}
	if _, err := normalizeBulkIDs([]string{"../../etc/passwd"}); err == nil {
		t.Fatal("se esperaba error para id invalido")
	}
}
