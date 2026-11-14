// Package config parses the server's environment-variable configuration.
package config

import (
	"errors"
	"fmt"
	"strings"
)

// Config is everything the server needs from its environment.
type Config struct {
	Host        string // PVE_HOST, e.g. https://pve.example.com (no trailing slash)
	TokenID     string // PVE_TOKEN_ID, e.g. pve-mcp@pve!mcp
	TokenSecret string // PVE_TOKEN_SECRET
	Pool        string // PVE_SANDBOX_POOL: the only pool the server may touch
	CAFile      string // PVE_CA_FILE: optional extra CA bundle (PEM)
	AllowWrite  bool   // ALLOW_WRITE=true enables start/shutdown/snapshot tools
}

// Load builds a Config from a getenv function (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Host:        strings.TrimRight(getenv("PVE_HOST"), "/"),
		TokenID:     getenv("PVE_TOKEN_ID"),
		TokenSecret: getenv("PVE_TOKEN_SECRET"),
		Pool:        getenv("PVE_SANDBOX_POOL"),
		CAFile:      getenv("PVE_CA_FILE"),
		AllowWrite:  getenv("ALLOW_WRITE") == "true",
	}
	var missing []string
	for _, f := range []struct{ name, val string }{
		{"PVE_HOST", c.Host},
		{"PVE_TOKEN_ID", c.TokenID},
		{"PVE_TOKEN_SECRET", c.TokenSecret},
		{"PVE_SANDBOX_POOL", c.Pool},
	} {
		if f.val == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	if !strings.HasPrefix(c.Host, "https://") {
		return Config{}, errors.New("PVE_HOST must start with https://")
	}
	return c, nil
}

// AuthHeader is the value for the Authorization header.
func (c Config) AuthHeader() string {
	return "PVEAPIToken=" + c.TokenID + "=" + c.TokenSecret
}
