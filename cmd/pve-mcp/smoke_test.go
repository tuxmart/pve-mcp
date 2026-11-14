//go:build smoke

package main

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestSmoke runs the real binary over stdio and checks initialize + tools/list.
// It uses dummy credentials: nothing here contacts PVE.
func TestSmoke(t *testing.T) {
	for _, tc := range []struct {
		allowWrite string
		wantTools  []string
		absent     []string
	}{
		{"", []string{"list_vms", "vm_status", "snapshot_list"}, []string{"start_vm", "shutdown_vm", "snapshot_create"}},
		{"true", []string{"list_vms", "vm_status", "snapshot_list", "start_vm", "shutdown_vm", "snapshot_create"}, nil},
	} {
		t.Run("ALLOW_WRITE="+tc.allowWrite, func(t *testing.T) {
			cmd := exec.Command("go", "run", ".")
			cmd.Env = append(cmd.Environ(),
				"PVE_HOST=https://pve.invalid", "PVE_TOKEN_ID=u@pve!t", "PVE_TOKEN_SECRET=x",
				"PVE_SANDBOX_POOL=p", "ALLOW_WRITE="+tc.allowWrite)
			in, _ := cmd.StdinPipe()
			out, _ := cmd.StdoutPipe()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()

			r := bufio.NewReader(out)
			send := func(s string) { _, _ = in.Write([]byte(s + "\n")) }
			read := func() string {
				done := make(chan string, 1)
				go func() { l, _ := r.ReadString('\n'); done <- l }()
				select {
				case l := <-done:
					return l
				case <-time.After(60 * time.Second):
					t.Fatal("timeout waiting for server")
					return ""
				}
			}

			send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}`)
			if !json.Valid([]byte(read())) {
				t.Fatal("initialize reply is not JSON")
			}
			send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
			send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
			list := read()
			for _, n := range tc.wantTools {
				if !strings.Contains(list, `"`+n+`"`) {
					t.Errorf("tool %s missing from %s", n, list)
				}
			}
			for _, n := range tc.absent {
				if strings.Contains(list, `"`+n+`"`) {
					t.Errorf("tool %s must not be registered without ALLOW_WRITE", n)
				}
			}
		})
	}
}
