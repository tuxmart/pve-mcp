// Package tools holds the MCP tool implementations. Service methods are the
// pure core (typed input -> text); register.go wires them to the MCP server.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/tuxmart/pve-mcp/internal/pve"
)

var snapshotName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

// Service implements the tools on top of a pool-confined PVE client.
type Service struct {
	PVE *pve.Client
}

// ListVMs returns the pool's VMs as JSON.
func (s *Service) ListVMs(ctx context.Context) (string, error) {
	ms, err := s.PVE.Members(ctx)
	if err != nil {
		return "", err
	}
	if ms == nil {
		ms = []pve.Member{}
	}
	b, err := json.Marshal(ms)
	return string(b), err
}

// VMStatus returns the current status JSON of a VM in the pool.
func (s *Service) VMStatus(ctx context.Context, vmid int) (string, error) {
	node, err := s.PVE.Locate(ctx, vmid)
	if err != nil {
		return "", err
	}
	var raw json.RawMessage
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid)
	if err := s.PVE.Call(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return "", err
	}
	return string(raw), nil
}

// Power runs start or shutdown on a VM in the pool and waits for the task.
func (s *Service) Power(ctx context.Context, vmid int, action string) (string, error) {
	if action != "start" && action != "shutdown" {
		return "", fmt.Errorf("unsupported action %q", action)
	}
	node, err := s.PVE.Locate(ctx, vmid)
	if err != nil {
		return "", err
	}
	var upid string
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", url.PathEscape(node), vmid, action)
	if err := s.PVE.Call(ctx, http.MethodPost, path, url.Values{}, &upid); err != nil {
		return "", err
	}
	res, err := s.PVE.Wait(ctx, node, upid, 60*time.Second)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s vm %d: %s (upid %s)", action, vmid, res, upid), nil
}

// SnapshotCreate snapshots a VM in the pool and waits for the task.
func (s *Service) SnapshotCreate(ctx context.Context, vmid int, name string) (string, error) {
	if !snapshotName.MatchString(name) {
		return "", fmt.Errorf("invalid snapshot name %q: use 1-40 chars of [A-Za-z0-9_-]", name)
	}
	node, err := s.PVE.Locate(ctx, vmid)
	if err != nil {
		return "", err
	}
	var upid string
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(node), vmid)
	if err := s.PVE.Call(ctx, http.MethodPost, path, url.Values{"snapname": {name}}, &upid); err != nil {
		return "", err
	}
	res, err := s.PVE.Wait(ctx, node, upid, 120*time.Second)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("snapshot %q of vm %d: %s (upid %s)", name, vmid, res, upid), nil
}

// SnapshotList returns the snapshot list JSON of a VM in the pool.
func (s *Service) SnapshotList(ctx context.Context, vmid int) (string, error) {
	node, err := s.PVE.Locate(ctx, vmid)
	if err != nil {
		return "", err
	}
	var raw json.RawMessage
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(node), vmid)
	if err := s.PVE.Call(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return "", err
	}
	return string(raw), nil
}
