package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSecretOrEnv_Literal(t *testing.T) {
	t.Setenv("MY_KEY", "literal-value")

	if got := getSecretOrEnv("MY_KEY"); got != "literal-value" {
		t.Fatalf("got %q, want %q", got, "literal-value")
	}
}

func TestGetSecretOrEnv_SecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("  file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MY_KEY", path)

	if got := getSecretOrEnv("MY_KEY"); got != "file-value" {
		t.Fatalf("got %q, want %q (should read+trim the secret file)", got, "file-value")
	}
}

func TestGetSecretOrEnv_NonexistentPathTreatedAsLiteral(t *testing.T) {
	// A "/"-prefixed value whose file does not exist falls through to being
	// returned as a literal rather than failing.
	t.Setenv("MY_KEY", "/no/such/file/here")

	if got := getSecretOrEnv("MY_KEY"); got != "/no/such/file/here" {
		t.Fatalf("got %q, want the literal path back", got)
	}
}
