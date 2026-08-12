package flutter_api

import (
	"os"
	"path/filepath"
	"testing"
)

// mkProjectRoot 造一个含 go.mod + backend/ 的假项目根
func mkProjectRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolveT0CacheRoot_EnvOverride(t *testing.T) {
	env := t.TempDir()
	got, err := resolveT0CacheRoot(env, "/nonexistent/cwd", "/nonexistent/exe")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(env) {
		t.Fatalf("got %q want %q", got, filepath.Clean(env))
	}
}

func TestResolveT0CacheRoot_FromCwd(t *testing.T) {
	root := mkProjectRoot(t)
	nested := filepath.Join(root, "cmd", "server")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveT0CacheRoot("", nested, "/nonexistent/exe")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "backend", "data", "cache")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveT0CacheRoot_FromExeDir(t *testing.T) {
	root := mkProjectRoot(t)
	got, err := resolveT0CacheRoot("", "/nonexistent/cwd", root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "backend", "data", "cache")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveT0CacheRoot_NotFound(t *testing.T) {
	if _, err := resolveT0CacheRoot("", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("无法定位项目根时应返回错误")
	}
}
