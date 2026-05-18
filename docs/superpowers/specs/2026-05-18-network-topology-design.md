# Network Topology Command Design

## Overview

Add `hlctl network topology` as a subcommand under `network`. It calls the new
`GET /network/topology` API endpoint and renders the result as an ASCII tree
rooted at the gateway, or as raw JSON.

## Command Interface

```
hlctl network topology [flags]

Flags:
  --include-clients    Include wired clients in the topology tree
  --include-wireless   Also include wireless clients (requires --include-clients)
  -o, --output         Output format: table|json (default: table)
```

- Default (no flags): devices only — gateway, switches, access points.
- `--include-clients`: sends `includeClients=true` to the API; renders wired
  clients only (wireless nodes and edges filtered client-side).
- `--include-wireless`: also renders wireless clients; implies
  `--include-clients` (passing it alone is not an error).
- `--output json`: prints the raw API response body unchanged.

## Table Output Format

ASCII tree rooted at the gateway, indented with box-drawing connectors:

```
USG (gateway)
├── Switch Living Room (switch) [port 1, 1 GbE]
│   ├── AP Living Room (accessPoint) [port 7, 2.5 GbE]  [3 clients]
│   │   └── MacBook Pro (client, wireless, HomeNetwork, -55 dBm)
│   └── nas-1 (client, wired) [port 8, 2.5 GbE]
```

Each node line includes:
- Name and type (in parentheses)
- For device→device wired edges: port number and link speed in brackets
- For AP device nodes: `[N clients]` when `numClients` is present and > 0
- For wired client edges: port and link speed when present (omitted for offline)
- For wireless client edges: SSID and signal strength when present (signal
  omitted for offline clients)

## Architecture

Files touched:

| File | Change |
|------|--------|
| `oapi-codegen-network.yaml` | No change needed — existing config picks up new operations |
| `internal/network/api.gen.go` | Regenerated via `make generate` |
| `internal/cli/network/client.go` | Add `GetNetworkTopology` to `NetworkClient` interface |
| `internal/cli/network/stub.go` | Add `GetNetworkTopologyFunc` to `StubClient` |
| `internal/cli/network/network.go` | Add `newTopologyCmd()`, register on parent, tree rendering logic |
| `internal/cli/network/network_test.go` | Add topology command tests |

## Data Flow

1. Parse `--include-clients` and `--include-wireless` flags.
2. If either flag is set (`--include-wireless` implies `--include-clients`),
   set `includeClients=true` in query params; otherwise omit.
3. Call `GetNetworkTopology(ctx, params)`.
4. On non-200 response, return `apiclient.ParseError(resp)`.
5. Unmarshal into `gen.NetworkTopology`.
6. For JSON output: print raw body.
7. For table output:
   a. If `--include-wireless` is absent, remove wireless edges and their
      source client nodes from the in-memory graph.
   b. Build adjacency map: `nodeID → []childEdge` where a child of X is any
      node whose edge points to X as target.
   c. Find the root: device node with `type == gateway`.
   d. Recursively render the tree with `├──`/`└──`/`│ ` prefix tracking.

## Tree Rendering Algorithm

```
func renderTree(w, node, edges, prefix, isLast):
  connector = "└── " if isLast else "├── "
  print(prefix + connector + formatNode(node, edge))
  childPrefix = prefix + ("    " if isLast else "│   ")
  children = adjacency[node.id]
  for i, child in enumerate(children):
    renderTree(w, child.node, edges, childPrefix, i == len(children)-1)
```

Root node is printed without a connector (no parent prefix).

## Testing

Table-driven tests using `StubClient`:

| Test case | Flags | Expected |
|-----------|-------|----------|
| Devices only | none | Tree with gateway, switch, AP; no clients |
| Wired clients | `--include-clients` | Wired clients shown; wireless filtered out |
| All clients | `--include-clients --include-wireless` | Both wired and wireless clients shown |
| JSON passthrough | `--output json` | Raw body printed, no tree rendering |
| API error | 401 response | Error returned containing status message |

## Error Handling

Follows existing command pattern:
- HTTP transport errors: returned directly from client call.
- Non-200 HTTP status: `apiclient.ParseError(resp)` extracts the problem detail.
- Unknown discriminator values: return `fmt.Errorf("unknown kind: %s", disc)`.
