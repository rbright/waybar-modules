# github

Waybar module backend for GitHub pull requests.

Builds binary: `waybar-github`

## Build/run (from monorepo root)

```bash
nix build 'path:.#github'
nix run 'path:.#github' -- status
```

## Build/test (from this directory)

```bash
go test ./...
go build ./cmd/waybar-github
```

## Usage

```bash
waybar-github <status|refresh|open-dashboard|open-item N>
```
