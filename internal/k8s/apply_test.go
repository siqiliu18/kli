package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectYAMLFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "deploy.yaml")
	os.WriteFile(f, []byte("kind: Deployment"), 0644)

	files, err := collectYAMLFiles(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0] != f {
		t.Errorf("got %v, want [%s]", files, f)
	}
}

func TestCollectYAMLFiles_YmlExtension(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "deploy.yml")
	os.WriteFile(f, []byte("kind: Deployment"), 0644)

	files, err := collectYAMLFiles(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("got %v, want 1 file", files)
	}
}

func TestCollectYAMLFiles_Directory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("kind: Deployment"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yml"), []byte("kind: Service"), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644)

	files, err := collectYAMLFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (txt should be skipped): %v", len(files), files)
	}
}

func TestCollectYAMLFiles_Nested(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(dir, "top.yaml"), []byte("kind: Deployment"), 0644)
	os.WriteFile(filepath.Join(sub, "nested.yaml"), []byte("kind: Service"), 0644)

	files, err := collectYAMLFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("got %d files, want 2 (recursive walk): %v", len(files), files)
	}
}

func TestCollectYAMLFiles_NotFound(t *testing.T) {
	_, err := collectYAMLFiles("/nonexistent/path/deploy.yaml")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}
