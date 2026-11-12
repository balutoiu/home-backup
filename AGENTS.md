# Agent Development Guide

A file for [guiding coding agents](https://agents.md/).

## Commands

- **Build:** `make build`
- **Test:** `make test`
- **Lint:** `make lint`
- **Format:** `make fmt`
- **Clean:** `make clean`

## Essential Constraints

- This is a Linux-only application. On non-Linux hosts, Make runs Go commands in a Linux Docker container; Docker is required.
- Sources and destinations are compile-time adapters wired in `internal/app`. Do not introduce runtime plugin registries or dependency-injection frameworks.
- `internal/config` owns YAML decoding and every user-settable default. Adapter packages (`internal/lvm`, `internal/restic`, …) take a complete plain `Config` and use it as given.

## Verification

After making changes, run these steps in order:

1. `make fmt` — format code
2. `make build` — ensure it compiles
3. `make test` — all tests pass
4. `make lint` — no lint errors
5. `make test-lvm-vm` — only for changes to `internal/lvm`, `internal/command`, `internal/app` or the `Dockerfile`; runs the LVM scenarios in throwaway libvirt VMs (needs KVM and libvirt, takes minutes)
