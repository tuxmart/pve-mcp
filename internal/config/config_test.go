package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	full := map[string]string{
		"PVE_HOST":         "https://pve.example.com/",
		"PVE_TOKEN_ID":     "pve-mcp@pve!mcp",
		"PVE_TOKEN_SECRET": "s3cret",
		"PVE_SANDBOX_POOL": "pve-mcp-sandbox",
	}
	with := func(k, v string) map[string]string {
		m := map[string]string{}
		for kk, vv := range full {
			m[kk] = vv
		}
		m[k] = v
		return m
	}

	tests := []struct {
		name      string
		env       map[string]string
		wantErr   string
		wantWrite bool
	}{
		{name: "ok, read-only by default", env: full},
		{name: "write opt-in", env: with("ALLOW_WRITE", "true"), wantWrite: true},
		{name: "write needs exactly true", env: with("ALLOW_WRITE", "1")},
		{name: "missing secret", env: with("PVE_TOKEN_SECRET", ""), wantErr: "PVE_TOKEN_SECRET"},
		{name: "missing pool", env: with("PVE_SANDBOX_POOL", ""), wantErr: "PVE_SANDBOX_POOL"},
		{name: "plain http rejected", env: with("PVE_HOST", "http://pve.example.com"), wantErr: "https://"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Load(env(tc.env))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.AllowWrite != tc.wantWrite {
				t.Errorf("AllowWrite = %v, want %v", c.AllowWrite, tc.wantWrite)
			}
			if c.Host != "https://pve.example.com" {
				t.Errorf("Host = %q, trailing slash not trimmed", c.Host)
			}
			if got, want := c.AuthHeader(), "PVEAPIToken=pve-mcp@pve!mcp=s3cret"; got != want {
				t.Errorf("AuthHeader = %q, want %q", got, want)
			}
		})
	}
}
