# schedule

Waybar module backend for upcoming meetings (EDS / GNOME Online Accounts).

Builds binary: `waybar-schedule`

## Build/run (from monorepo root)

```bash
nix build 'path:.#schedule'
nix run 'path:.#schedule' -- status
```

## Build/test (from this directory)

```bash
go test ./...
go build ./cmd/waybar-schedule
```

## Usage

```bash
waybar-schedule <status|refresh|join-next|join-item N|select-calendars>
```
