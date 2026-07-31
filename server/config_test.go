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

// getOptionalSecretOrEnv must return "" for an unset key rather than calling
// os.Exit, which is what makes NOTIFICATION_URLS optional.
func TestGetOptionalSecretOrEnv_UnsetReturnsEmpty(t *testing.T) {
	t.Setenv("MY_OPTIONAL_KEY", "")

	if got := getOptionalSecretOrEnv("MY_OPTIONAL_KEY"); got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestGetOptionalSecretOrEnv_SecretFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("  file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MY_OPTIONAL_KEY", path)

	if got := getOptionalSecretOrEnv("MY_OPTIONAL_KEY"); got != "file-value" {
		t.Fatalf("got %q, want %q (should read+trim the secret file)", got, "file-value")
	}
}

func TestParseNotificationURLs(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "empty",
			value: "",
			want:  []string{},
		},
		{
			name:  "single url",
			value: "generic://example.com/hook",
			want:  []string{"generic://example.com/hook"},
		},
		{
			name:  "comma separated with spaces",
			value: "generic://example.com/hook, discord://token@id",
			want:  []string{"generic://example.com/hook", "discord://token@id"},
		},
		{
			name:  "newline separated, as a secret file would be",
			value: "generic://example.com/hook\ndiscord://token@id\n",
			want:  []string{"generic://example.com/hook", "discord://token@id"},
		},
		{
			name:  "trailing comma and blank lines are dropped",
			value: "generic://example.com/hook,\n\n,",
			want:  []string{"generic://example.com/hook"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNotificationURLs(tt.value)

			if len(got) != len(tt.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("url %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
