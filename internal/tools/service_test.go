package tools

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tuxmart/pve-mcp/internal/pve"
)

// fakePVE serves a pool with one VM (100) and records every request path.
type fakePVE struct {
	srv  *httptest.Server
	mu   sync.Mutex
	hits []string
}

func newFakePVE(t *testing.T) (*fakePVE, *Service) {
	t.Helper()
	f := &fakePVE{}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits = append(f.hits, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "PVEAPIToken=u@pve!t=secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/api2/json/pools/sandbox":
			_, _ = w.Write([]byte(`{"data":{"members":[
				{"type":"qemu","vmid":100,"node":"pve","name":"vm100","status":"stopped"},
				{"type":"lxc","vmid":101,"node":"pve","name":"ct101","status":"running"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			_, _ = w.Write([]byte(`{"data":{"status":"stopped"}}`))
		case strings.HasSuffix(r.URL.Path, "/status/start"), strings.HasSuffix(r.URL.Path, "/snapshot") && r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"data":"UPID:pve:1"}`))
		case strings.Contains(r.URL.Path, "/tasks/"):
			_, _ = w.Write([]byte(`{"data":{"status":"stopped","exitstatus":"OK"}}`))
		case strings.HasSuffix(r.URL.Path, "/snapshot"):
			_, _ = w.Write([]byte(`{"data":[{"name":"current"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)

	ca := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.srv.Certificate().Raw})
	if err := os.WriteFile(ca, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := pve.New(f.srv.URL, "PVEAPIToken=u@pve!t=secret", "sandbox", ca)
	if err != nil {
		t.Fatal(err)
	}
	return f, &Service{PVE: c}
}

func (f *fakePVE) hitsContaining(s string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, h := range f.hits {
		if strings.Contains(h, s) {
			n++
		}
	}
	return n
}

func TestListVMsOnlyQemu(t *testing.T) {
	_, svc := newFakePVE(t)
	got, err := svc.ListVMs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"vmid":100`) || strings.Contains(got, "101") {
		t.Errorf("want only qemu vm 100, got %s", got)
	}
}

func TestAllowlistBlocksVMOutsidePool(t *testing.T) {
	f, svc := newFakePVE(t)
	ctx := context.Background()
	calls := map[string]func() error{
		"status":   func() error { _, err := svc.VMStatus(ctx, 200); return err },
		"start":    func() error { _, err := svc.Power(ctx, 200, "start"); return err },
		"snapshot": func() error { _, err := svc.SnapshotCreate(ctx, 200, "x"); return err },
		"snaplist": func() error { _, err := svc.SnapshotList(ctx, 200); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil || !strings.Contains(err.Error(), "not in pool") {
			t.Errorf("%s: want 'not in pool' error, got %v", name, err)
		}
	}
	if n := f.hitsContaining("/qemu/200"); n != 0 {
		t.Errorf("server saw %d requests for the VM outside the pool, want 0", n)
	}
}

func TestPowerStartWaitsForTask(t *testing.T) {
	_, svc := newFakePVE(t)
	got, err := svc.Power(context.Background(), 100, "start")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "start vm 100: OK") {
		t.Errorf("got %q", got)
	}
}

func TestPowerRejectsOtherActions(t *testing.T) {
	f, svc := newFakePVE(t)
	if _, err := svc.Power(context.Background(), 100, "stop"); err == nil {
		t.Fatal("want error for unsupported action")
	}
	if len(f.hits) != 0 {
		t.Errorf("unsupported action must not reach PVE, saw %v", f.hits)
	}
}

func TestSnapshotNameValidation(t *testing.T) {
	tests := []struct {
		name string
		ok   bool
	}{
		{"before-upgrade_1", true},
		{"", false},
		{"has space", false},
		{"semi;colon", false},
		{"../etc", false},
		{strings.Repeat("a", 41), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, svc := newFakePVE(t)
			_, err := svc.SnapshotCreate(context.Background(), 100, tc.name)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatal("want error")
				}
				if len(f.hits) != 0 {
					t.Errorf("invalid name must be rejected before any request, saw %v", f.hits)
				}
			}
		})
	}
}

func TestBadTokenSurfacesAsError(t *testing.T) {
	f, _ := newFakePVE(t)
	c, err := pve.New(f.srv.URL, "PVEAPIToken=wrong", "sandbox", "")
	if err != nil {
		t.Fatal(err)
	}
	// No CA configured, so the self-signed fake is rejected: proves TLS is verified.
	if _, err := (&Service{PVE: c}).ListVMs(context.Background()); err == nil {
		t.Fatal("want TLS verification error")
	}
}
