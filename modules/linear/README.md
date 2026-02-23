# linear

Waybar module backend for Linear unread notifications.

Builds binary: `waybar-linear`

## Build/run (from monorepo root)

```bash
nix build 'path:.#linear'
nix run 'path:.#linear' -- status
```

## Build/test (from this directory)

```bash
go test ./...
go build ./cmd/waybar-linear
```

## Usage

```bash
waybar-linear <status|refresh|open-inbox|mark-all-read|open-item N>
```
