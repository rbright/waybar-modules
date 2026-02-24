# sotto

Waybar module backend for selecting Sotto microphone input.

Builds binary: `waybar-sotto`

## Build/run (from monorepo root)

```bash
nix build 'path:.#sotto'
nix run 'path:.#sotto' -- status
```

## Build/test (from this directory)

```bash
go test ./...
go build ./cmd/waybar-sotto
```

## Usage

```bash
waybar-sotto <status|refresh|select-item N|select-input>
```

`select-input` opens a compact `fuzzel` dmenu selector.

It uses `hyprctl` cursor/monitor geometry to place the selector near the click/pointer location on the same monitor, and applies Catppuccin Mocha + IBM Plex Sans styling for desktop consistency.
