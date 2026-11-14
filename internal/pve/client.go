// Package pve is a small Proxmox VE API client confined to one resource pool.
package pve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to the PVE API with an API token. Every VM operation goes
// through Locate, so only VMs that are members of Pool are reachable.
type Client struct {
	base, auth, pool string
	hc               *http.Client
}

// New builds a client. caFile, if non-empty, adds extra trusted roots on top
// of the system pool.
func New(base, auth, pool, caFile string) (*Client, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		cp, err := x509.SystemCertPool()
		if err != nil || cp == nil {
			cp = x509.NewCertPool()
		}
		if !cp.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no certificates found in %s", caFile)
		}
		tr.TLSClientConfig.RootCAs = cp
	}
	return &Client{
		base: base,
		auth: auth,
		pool: pool,
		hc:   &http.Client{Transport: tr, Timeout: 30 * time.Second},
	}, nil
}

// Pool returns the pool this client is confined to.
func (c *Client) Pool() string { return c.pool }

// Call performs an API request and decodes the "data" envelope into out.
func (c *Client) Call(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+"/api2/json"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("pve %s: %s", resp.Status, b)
	}
	if out == nil {
		return nil
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		return err
	}
	return json.Unmarshal(env.Data, out)
}

// Member is a guest in the pool.
type Member struct {
	Type   string `json:"type"`
	VMID   int    `json:"vmid"`
	Node   string `json:"node"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Members lists the QEMU VMs in the pool.
func (c *Client) Members(ctx context.Context) ([]Member, error) {
	var d struct {
		Members []Member `json:"members"`
	}
	if err := c.Call(ctx, http.MethodGet, "/pools/"+url.PathEscape(c.pool), nil, &d); err != nil {
		return nil, err
	}
	var vms []Member
	for _, m := range d.Members {
		if m.Type == "qemu" {
			vms = append(vms, m)
		}
	}
	return vms, nil
}

// Locate is the allowlist: it returns the node of a VM only if vmid is in the pool.
func (c *Client) Locate(ctx context.Context, vmid int) (string, error) {
	ms, err := c.Members(ctx)
	if err != nil {
		return "", err
	}
	for _, m := range ms {
		if m.VMID == vmid {
			return m.Node, nil
		}
	}
	return "", fmt.Errorf("vmid %d is not in pool %q", vmid, c.pool)
}

// Wait polls a task until it stops or the timeout passes. It returns the task's
// exit status ("OK" on success) or a note that the task is still running.
func (c *Client) Wait(ctx context.Context, node, upid string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		var st struct{ Status, Exitstatus string }
		path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))
		if err := c.Call(ctx, http.MethodGet, path, nil, &st); err != nil {
			return "", err
		}
		if st.Status == "stopped" {
			return st.Exitstatus, nil
		}
		if time.Now().After(deadline) {
			return "still running after " + timeout.String() + "; check the task in the PVE UI", nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
