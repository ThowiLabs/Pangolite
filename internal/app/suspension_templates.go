package app

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type SuspensionTemplate struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`
	HTML      string    `json:"html,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type SuspensionTemplateVars struct {
	Resource Resource
	Project  Project
	Status   int
	Reason   string
	Now      time.Time
}

var dangerousHTMLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<\s*script\b`),
	regexp.MustCompile(`(?is)<\s*iframe\b`),
	regexp.MustCompile(`(?is)<\s*object\b`),
	regexp.MustCompile(`(?is)<\s*embed\b`),
	regexp.MustCompile(`(?is)<\s*svg\b`),
	regexp.MustCompile(`(?is)<\s*form\b`),
	regexp.MustCompile(`(?is)<\s*input\b`),
	regexp.MustCompile(`(?is)<\s*button\b`),
	regexp.MustCompile(`(?is)<\s*meta\b[^>]*http-equiv\s*=\s*["']?refresh`),
	regexp.MustCompile(`(?is)\s+on[a-z0-9_-]+\s*=`),
	regexp.MustCompile(`(?is)(javascript|vbscript)\s*:`),
	regexp.MustCompile(`(?is)data\s*:\s*text/html`),
}

func EnsureSuspensionTemplates(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("directorio de plantillas requerido")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio de plantillas: %w", err)
	}
	for id, content := range defaultSuspensionTemplates() {
		path := filepath.Join(dir, id+".html")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("crear plantilla %s: %w", id, err)
			}
		}
	}
	return nil
}

func ListSuspensionTemplates(dir string) ([]SuspensionTemplate, error) {
	if err := EnsureSuspensionTemplates(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("leer plantillas: %w", err)
	}
	out := []SuspensionTemplate{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".html")
		if !templateIDRe.MatchString(id) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, _ := entry.Info()
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := templateNameFromHTML(id, string(b))
		updated := time.Time{}
		if info != nil {
			updated = info.ModTime().UTC()
		}
		out = append(out, SuspensionTemplate{ID: id, Name: name, Path: path, UpdatedAt: updated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func ReadSuspensionTemplate(dir, id string) (SuspensionTemplate, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !templateIDRe.MatchString(id) {
		return SuspensionTemplate{}, errors.New("id de plantilla invalido")
	}
	if err := EnsureSuspensionTemplates(dir); err != nil {
		return SuspensionTemplate{}, err
	}
	path := filepath.Join(dir, id+".html")
	b, err := os.ReadFile(path)
	if err != nil {
		return SuspensionTemplate{}, errors.New("plantilla no encontrada")
	}
	info, _ := os.Stat(path)
	updated := time.Time{}
	if info != nil {
		updated = info.ModTime().UTC()
	}
	html := string(b)
	return SuspensionTemplate{ID: id, Name: templateNameFromHTML(id, html), Path: path, HTML: html, UpdatedAt: updated}, nil
}

func SaveSuspensionTemplate(dir, id, content string) (SuspensionTemplate, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !templateIDRe.MatchString(id) {
		return SuspensionTemplate{}, errors.New("id de plantilla invalido")
	}
	content = strings.TrimSpace(content)
	if err := ValidateSuspensionHTML(content); err != nil {
		return SuspensionTemplate{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SuspensionTemplate{}, fmt.Errorf("crear directorio de plantillas: %w", err)
	}
	path := filepath.Join(dir, id+".html")
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		return SuspensionTemplate{}, fmt.Errorf("guardar plantilla: %w", err)
	}
	return ReadSuspensionTemplate(dir, id)
}

func ValidateSuspensionHTML(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("HTML requerido")
	}
	if len(content) > 131072 {
		return errors.New("HTML no debe superar 128 KB")
	}
	lower := strings.ToLower(content)
	for _, re := range dangerousHTMLPatterns {
		if re.MatchString(lower) {
			return errors.New("HTML rechazado por contener etiquetas, atributos o URLs potencialmente peligrosas")
		}
	}
	return nil
}

func RenderSuspensionHTML(content string, vars SuspensionTemplateVars) string {
	if vars.Status == 0 {
		vars.Status = 403
	}
	if vars.Now.IsZero() {
		vars.Now = time.Now().UTC()
	}
	resourceName := vars.Resource.Name
	projectName := vars.Project.Name
	if projectName == "" {
		projectName = vars.Resource.ProjectID
	}
	if vars.Reason == "" {
		vars.Reason = "Servicio no disponible temporalmente"
	}
	repl := map[string]string{
		"$nombredominio": html.EscapeString(vars.Resource.Domain),
		"$dominio":       html.EscapeString(vars.Resource.Domain),
		"$nombrerecurso": html.EscapeString(resourceName),
		"$recurso":       html.EscapeString(resourceName),
		"$proyecto":      html.EscapeString(projectName),
		"$codigo":        fmt.Sprint(vars.Status),
		"$motivo":        html.EscapeString(vars.Reason),
		"$fecha":         html.EscapeString(vars.Now.Format("2006-01-02 15:04 MST")),
	}
	keys := make([]string, 0, len(repl)*2)
	for k, v := range repl {
		keys = append(keys, k, v)
	}
	return strings.NewReplacer(keys...).Replace(content)
}

func templateNameFromHTML(id, content string) string {
	m := regexp.MustCompile(`(?is)<title>(.*?)</title>`).FindStringSubmatch(content)
	if len(m) > 1 {
		name := strings.TrimSpace(m[1])
		if name != "" && len(name) <= 80 {
			return name
		}
	}
	return strings.Title(strings.ReplaceAll(id, "-", " "))
}

func defaultSuspensionTemplates() map[string]string {
	out := map[string]string{}
	entries, err := fs.Glob(templatesFS, "templates/suspension_defaults/*.html")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		b, err := templatesFS.ReadFile(entry)
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(entry), ".html")
		if templateIDRe.MatchString(id) {
			out[id] = string(b)
		}
	}
	return out
}
