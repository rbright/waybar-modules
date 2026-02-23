# agent-usage

Waybar module backend for Codex + Claude usage.

Builds binary: `waybar-agent-usage`

## Build/run (from monorepo root)

```bash
nix build 'path:.#agent-usage'
nix run 'path:.#agent-usage' -- codex
```

## Build/test (from this directory)

```bash
go test ./...
go build ./cmd/waybar-agent-usage
```

## Usage

```bash
waybar-agent-usage <codex|claude> [--refresh]
```
