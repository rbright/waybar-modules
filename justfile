set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

mod modules

default:
    @just --list --list-submodules

fmt: modules::agent-usage::fmt modules::github::fmt modules::linear::fmt modules::schedule::fmt modules::sotto::fmt

fmt-check: modules::agent-usage::fmt-check modules::github::fmt-check modules::linear::fmt-check modules::schedule::fmt-check modules::sotto::fmt-check

test: modules::agent-usage::test modules::github::test modules::linear::test modules::schedule::test modules::sotto::test

lint: modules::agent-usage::lint modules::github::lint modules::linear::lint modules::schedule::lint modules::sotto::lint

build: modules::agent-usage::build modules::github::build modules::linear::build modules::schedule::build modules::sotto::build

ci-check: fmt-check test lint build

nix-build: modules::agent-usage::nix-build modules::github::nix-build modules::linear::nix-build modules::schedule::nix-build modules::sotto::nix-build
    nix build --no-link 'path:.#waybar-modules'

precommit-install:
    pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push

precommit-run:
    pre-commit run --all-files --hook-stage pre-commit

prepush-run:
    pre-commit run --all-files --hook-stage pre-push
