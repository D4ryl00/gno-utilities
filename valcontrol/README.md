# valcontrol

`valcontrol` is an interactive terminal UI for live control of validators launched with the controllable signer sidecars from `gno/misc/val-scenarios`.

It reads the `inventory.json` inventory emitted by `prepare_network` and talks to:

- validator RPC endpoints for status
- signer sidecar control APIs for live fault injection

## Build

```bash
go build ./cmd/valcontrol
```

## Inventory

Point `valcontrol` at the inventory file produced by a running scenario:

```bash
export INVENTORY_PATH=/tmp/gno-val-tests/scenario-14/inventory.json
```

Or pass it explicitly with `--inventory`.

## Run

```bash
valcontrol
```

Or explicitly:

```bash
valcontrol tui
```

The default screen shows validators on the left and live details for the selected node on the right.

## TUI Keys

```text
up/down  select validator
tab      hide / show details panel
space    toggle validator selection
a        select all / clear selection
r        refresh now
p        toggle proposal drop on rule targets
v        toggle prevote drop on rule targets
c        toggle precommit drop on rule targets
P        set proposal delay on rule targets
V        set prevote delay on rule targets
C        set precommit delay on rule targets
x        clear all rules on rule targets
q        quit
```

When no validators are explicitly selected, rule actions target the focused row. When one or more validators are selected, rule actions target the selected validators together.

## Bootstrap a New Chain

Create and start a chain with N validators non-interactively:

```bash
valcontrol new <count> [--name <name>] [--scenario-lib <path>] [--controllable-signer]
```

The inventory is written to `${WORK_ROOT:-/tmp/gno-val-tests}/<name>/inventory.json`.

## Optional Non-Interactive Commands

```bash
valcontrol list
valcontrol watch
valcontrol state val4
valcontrol drop val4 precommit
valcontrol delay val4 prevote 5s --height 25 --round 0
valcontrol clear val4 precommit
valcontrol clear val4
```

## Notes

This tool only controls the signer sidecars. It can drop or delay proposal / prevote / precommit signatures, but it does not rewrite vote contents or block contents.
