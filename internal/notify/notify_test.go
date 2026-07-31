package notify

import "testing"

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		urls        []string
		wantErr     bool
		wantEnabled bool
		wantCount   int
	}{
		{
			name:        "no urls is disabled, not an error",
			urls:        nil,
			wantEnabled: false,
		},
		{
			name:        "single generic webhook",
			urls:        []string{"generic://example.com/hook"},
			wantEnabled: true,
			wantCount:   1,
		},
		{
			name:        "multiple services",
			urls:        []string{"generic://example.com/hook", "discord://token@id"},
			wantEnabled: true,
			wantCount:   2,
		},
		{
			name:    "unknown scheme fails at startup",
			urls:    []string{"notaservice://example.com"},
			wantErr: true,
		},
		{
			name:    "one bad url fails the whole set",
			urls:    []string{"generic://example.com/hook", "notaservice://example.com"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := New(tt.urls)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if n.Enabled() != tt.wantEnabled {
				t.Errorf("Enabled() = %v, want %v", n.Enabled(), tt.wantEnabled)
			}
			if n.Services() != tt.wantCount {
				t.Errorf("Services() = %d, want %d", n.Services(), tt.wantCount)
			}
		})
	}
}

func TestSendDisabledIsNoop(t *testing.T) {
	n, err := New(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := n.Send("TITLE", "message"); err != nil {
		t.Errorf("Send on a disabled notifier returned %v, want nil", err)
	}
}
