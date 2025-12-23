package authorization

import (
	"os"
	"path/filepath"
	"testing"
)

// helper to create a temp file with contents
func writeTempFile(t *testing.T, dir, pattern, content string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("CreateTemp error: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp error: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

func TestLoad_ValidYAML(t *testing.T) {
	// ensure clean state
	cfg = nil
	t.Cleanup(func() { cfg = nil })

	dir := t.TempDir()
	y := "" +
		"coarse-check:\n" +
		"  enabled: true\n" +
		"  validation-url: \"http://example.org/coarse\"\n" +
		"  resource-map:\n" +
		"    \"[/x]\": \"/target\"\n" +
		"finegrain-check:\n" +
		"  enabled: false\n"
	p := writeTempFile(t, dir, "auth-*.yaml", y)

	if err := Load(p); err != nil {
		t.Fatalf("Load valid yaml error: %v", err)
	}
	c := ConfigOrNil()
	if c == nil {
		t.Fatalf("ConfigOrNil returned nil after Load")
	}
	if !c.Coarse.Enabled || c.Coarse.ValidationURL != "http://example.org/coarse" {
		t.Fatalf("unexpected coarse url: %s", c.Coarse.ValidationURL)
	}
	if got := len(c.Coarse.ResourceMap); got != 1 {
		t.Fatalf("expected 1 resource-map entry, got %d", got)
	}
	if c.FineGrain.Enabled {
		t.Fatalf("expected finegrain disabled by test yaml")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	cfg = nil
	t.Cleanup(func() { cfg = nil })

	err := Load(filepath.Join(t.TempDir(), "not-exists.yaml"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if ConfigOrNil() != nil {
		t.Fatalf("expected cfg to remain nil on error")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	cfg = nil
	t.Cleanup(func() { cfg = nil })
	p := writeTempFile(t, t.TempDir(), "bad-*.yaml", "::: not yaml :::")
	if err := Load(p); err == nil {
		t.Fatalf("expected unmarshal error for invalid yaml")
	}
}

func TestLoad_NoValidationURLs(t *testing.T) {
	cfg = nil
	t.Cleanup(func() { cfg = nil })
	y := "coarse-check:\n  enabled: true\n  validation-url: \"\"\n\n" +
		"finegrain-check:\n  enabled: true\n  validation-url: \"\"\n"
	p := writeTempFile(t, t.TempDir(), "empty-*.yaml", y)
	if err := Load(p); err == nil {
		t.Fatalf("expected error when both validation_url are empty")
	}
}

func TestConfigOrNil_DefaultNilAndSet(t *testing.T) {
	// default nil
	old := cfg
	cfg = nil
	t.Cleanup(func() { cfg = old })
	if ConfigOrNil() != nil {
		t.Fatalf("expected nil config by default")
	}
	tmp := &Config{}
	cfg = tmp
	if ConfigOrNil() != tmp {
		t.Fatalf("expected ConfigOrNil to return the same pointer that was set")
	}
}
