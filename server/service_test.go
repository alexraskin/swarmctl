package server

import (
	"sort"
	"testing"
)

func TestExtractHostnames(t *testing.T) {
	s := &Server{}

	cases := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name:   "no labels",
			labels: map[string]string{},
			want:   []string{},
		},
		{
			name:   "single primary hostname",
			labels: map[string]string{"cloudflared.tunnel.hostname": "a.example.com"},
			want:   []string{"a.example.com"},
		},
		{
			name:   "comma-separated primary hostnames with spaces",
			labels: map[string]string{"cloudflared.tunnel.hostname": "a.example.com, b.example.com"},
			want:   []string{"a.example.com", "b.example.com"},
		},
		{
			name: "indexed .hostname labels",
			labels: map[string]string{
				"cloudflared.tunnel.0.hostname": "a.example.com",
				"cloudflared.tunnel.1.hostname": "b.example.com",
			},
			want: []string{"a.example.com", "b.example.com"},
		},
		{
			name: "primary plus indexed",
			labels: map[string]string{
				"cloudflared.tunnel.hostname":   "a.example.com",
				"cloudflared.tunnel.1.hostname": "b.example.com",
			},
			want: []string{"a.example.com", "b.example.com"},
		},
		{
			name: "ignores empty values and unrelated labels",
			labels: map[string]string{
				"cloudflared.tunnel.hostname":   "",
				"cloudflared.tunnel.1.hostname": "b.example.com",
				"unrelated":                     "x.example.com",
			},
			want: []string{"b.example.com"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.extractHostnames(tc.labels)

			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)

			if len(got) != len(want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("got %v, want %v", got, want)
				}
			}
		})
	}
}
