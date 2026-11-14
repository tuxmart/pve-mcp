package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds the tools to s. Mutating tools (start_vm, shutdown_vm,
// snapshot_create) are only registered when allowWrite is true.
func Register(s *server.MCPServer, svc *Service, allowWrite bool) {
	pool := svc.PVE.Pool()
	vmid := mcp.WithNumber("vmid", mcp.Required(), mcp.Description("VM ID (must be in pool "+pool+")"))
	readOnly := mcp.WithReadOnlyHintAnnotation(true)

	s.AddTool(mcp.NewTool("list_vms",
		mcp.WithDescription("List VMs in the sandbox pool"), readOnly),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.ListVMs(ctx))
		})

	s.AddTool(mcp.NewTool("vm_status",
		mcp.WithDescription("Current status of a VM in the pool"), vmid, readOnly),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.VMStatus(ctx, r.GetInt("vmid", 0)))
		})

	s.AddTool(mcp.NewTool("snapshot_list",
		mcp.WithDescription("List snapshots of a VM in the pool"), vmid, readOnly),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.SnapshotList(ctx, r.GetInt("vmid", 0)))
		})

	if !allowWrite {
		return
	}

	s.AddTool(mcp.NewTool("start_vm",
		mcp.WithDescription("Start a VM in the pool (mutating)"), vmid),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.Power(ctx, r.GetInt("vmid", 0), "start"))
		})

	s.AddTool(mcp.NewTool("shutdown_vm",
		mcp.WithDescription("Graceful ACPI shutdown of a VM in the pool (mutating)"), vmid),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.Power(ctx, r.GetInt("vmid", 0), "shutdown"))
		})

	s.AddTool(mcp.NewTool("snapshot_create",
		mcp.WithDescription("Create a snapshot of a VM in the pool (mutating)"), vmid,
		mcp.WithString("name", mcp.Required(), mcp.Description("Snapshot name, 1-40 chars of [A-Za-z0-9_-]"))),
		func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return result(svc.SnapshotCreate(ctx, r.GetInt("vmid", 0), r.GetString("name", "")))
		})
}

// result turns a Service return into a tool result; errors become tool errors,
// not protocol errors, so the model can read and react to them.
func result(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(text), nil
}
