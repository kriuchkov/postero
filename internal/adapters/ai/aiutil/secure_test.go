package aiutil

import "testing"

func TestEnsureHTTPS(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https ok", "https://api.openai.com/v1", false},
		{"https gemini", "https://generativelanguage.googleapis.com/v1beta", false},
		{"http loopback ok", "http://127.0.0.1:8080/v1", false},
		{"http localhost ok", "http://localhost/v1", false},
		{"http public rejected", "http://api.openai.com/v1", true},
		{"http public ip rejected", "http://8.8.8.8/v1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := EnsureHTTPS(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("EnsureHTTPS(%q) = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("EnsureHTTPS(%q) = %v, want nil", tc.url, err)
			}
		})
	}
}
