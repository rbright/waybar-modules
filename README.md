# waybar-modules

Monorepo for standalone Waybar module backends.

## Modules (prefix dropped)

All modules live under `modules/`:

- `modules/agent-usage` (builds `waybar-agent-usage`)
- `modules/github` (builds `waybar-github`)
- `modules/linear` (builds `waybar-linear`)
- `modules/schedule` (builds `waybar-schedule`)
- `modules/sotto` (builds `waybar-sotto`)

Shared tooling, CI, linting, flake packaging, and workspace config live at the repository root.

## Nix flake outputs

Per-module packages:

- `.#agent-usage`
- `.#github`
- `.#linear`
- `.#schedule`
- `.#sotto`

Aggregate package:

- `.#waybar-modules` (contains all binaries)

## Development

From repo root:

```bash
just fmt
just test
just lint
just ci-check
just nix-build
```

Run a module-scoped recipe through submodule imports:

```bash
just modules::linear::test
just modules::schedule::build
```

Build one package directly:

```bash
nix build 'path:.#linear'
```

Run one module directly:

```bash
nix run 'path:.#linear' -- status
```
