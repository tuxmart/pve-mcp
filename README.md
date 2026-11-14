# pve-mcp

An [MCP](https://modelcontextprotocol.io) server (stdio) that lets an AI client inspect and operate the VMs in **one Proxmox VE resource pool**.
Every VM operation first checks that the VMID is a member of the configured pool, and the API token should be granted rights on that pool path only, so the server cannot reach anything else.

## Tools

| Tool | Mutates | Needs `ALLOW_WRITE=true` |
|---|---|---|
| `list_vms` | no | no |
| `vm_status` | no | no |
| `snapshot_list` | no | no |
| `start_vm` | **yes** | yes |
| `shutdown_vm` (graceful ACPI) | **yes** | yes |
| `snapshot_create` | **yes** | yes |

Without `ALLOW_WRITE=true` the mutating tools are not registered at all.

## Configuration (environment)

See `.env.example`. Credentials are env-only; never commit `.env*`.

| Variable | Meaning |
|---|---|
| `PVE_HOST` | e.g. `https://pve.example.com` (must be https) |
| `PVE_TOKEN_ID` | e.g. `pve-mcp@pve!mcp` |
| `PVE_TOKEN_SECRET` | token secret |
| `PVE_SANDBOX_POOL` | the one pool the server may touch |
| `PVE_CA_FILE` | optional extra CA bundle (PEM) for private CAs |
| `ALLOW_WRITE` | `true` to enable start/shutdown/snapshot |

### Required PVE privileges (on `/pool/<pool>` only)

`Pool.Audit`, `VM.Audit`, `VM.PowerMgmt`, `VM.Snapshot`. Use a privilege-separated token.

## Build and check

```
make build
make check    # lint + tests (needs golangci-lint)
make smoke    # starts the binary, checks MCP initialize + tools/list over stdio
```

## Use with an MCP client

```
set -a; . ./.env.pve; set +a
go run ./cmd/pve-mcp
```

Logs go to stderr; stdout is the protocol channel.

## License

[MIT](LICENSE)
