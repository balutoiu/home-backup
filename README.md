# home-backup

`home-backup` is a Linux CLI for sequential directory and LVM snapshot backups to Restic.

## Usage

```sh
make build
RESTIC_PASSWORD='...' ./build/home-backup -config ./config.yaml
```

Directory backups can run as an ordinary user. LVM backups require root and the Linux LVM tools.

## Configuration

See [`examples/sample-config.yaml`](examples/sample-config.yaml).

Provide the Restic password and backend credentials through Restic's standard environment variables or configuration. Configuration decoding is strict; unknown fields and invalid values are rejected before any backup starts.
