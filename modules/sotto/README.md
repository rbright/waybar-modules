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

`select-input` opens a compact dmenu-style selector and prefers:

1. `fuzzel`
2. `wofi`
3. `bemenu`
4. `rofi`

