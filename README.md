# home-backup

`home-backup` is a Linux CLI for sequential directory and LVM snapshot backups to Restic.

## Usage

```sh
make build
RESTIC_PASSWORD='...' ./build/home-backup -config ./config.yaml
```

Directory backups can run as an ordinary user. LVM backups require root and the Linux LVM tools.

An LVM backup snapshots `<vg>/<lv>` as `<vg>/<lv>_backup_snapshot` and removes the snapshot when it finishes. If an interrupted run left that snapshot behind, the next run removes it and logs a warning. Any other volume with that name fails the backup and is left untouched.

## Configuration

See [`examples/sample-config.yaml`](examples/sample-config.yaml).

Provide the Restic password and backend credentials through Restic's standard environment variables or configuration. home-backup passes each `repo` to Restic through `RESTIC_REPOSITORY`, so it never appears in logs or process arguments. Configuration decoding is strict; unknown fields and invalid values are rejected before any backup starts.
