package app

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestProjectIDFromRequestPreservesTerminalContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	project, err := store.AddProject(Project{Name: "Proyecto Terminal"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-01"})
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{store: store}

	byProject := httptest.NewRequest("GET", "/terminal?projectId="+project.ID, nil)
	if got := s.projectIDFromRequest(byProject); got != project.ID {
		t.Fatalf("projectId desde query = %q, want %q", got, project.ID)
	}

	byAgent := httptest.NewRequest("GET", "/terminal?agentId="+agent.ID, nil)
	if got := s.projectIDFromRequest(byAgent); got != project.ID {
		t.Fatalf("projectId desde agentId = %q, want %q", got, project.ID)
	}
}
