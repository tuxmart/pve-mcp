// Command pve-mcp is an MCP server (stdio) for a Proxmox VE sandbox pool.
package main

import (
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/tuxmart/pve-mcp/internal/config"
	"github.com/tuxmart/pve-mcp/internal/pve"
	"github.com/tuxmart/pve-mcp/internal/tools"
)

var version = "dev"

func main() {
	// stdout is the MCP protocol channel; all logs go to stderr.
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	client, err := pve.New(cfg.Host, cfg.AuthHeader(), cfg.Pool, cfg.CAFile)
	if err != nil {
		log.Error("pve client", "err", err)
		os.Exit(1)
	}

	s := server.NewMCPServer("pve-mcp", version)
	tools.Register(s, &tools.Service{PVE: client}, cfg.AllowWrite)
	log.Info("starting", "pool", cfg.Pool, "host", cfg.Host, "write", cfg.AllowWrite)

	if err := server.ServeStdio(s); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
