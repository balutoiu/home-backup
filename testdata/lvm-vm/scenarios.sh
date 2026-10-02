#!/bin/bash
# Runs inside a throwaway VM as root. Sets up LVM on the spare disk like a
# k3s node, then runs home-backup in a privileged container like the backup
# CronJob. Each scenario uses its own LVs, config and Restic repos.
#
# Usage: scenarios.sh <image>
set -euo pipefail

IMAGE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DISK=/dev/vdb
VG=hbtest
WORK=/srv/hbtest
export RESTIC_PASSWORD=lvm-vm-test-password

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# make_origin creates an LV with a filesystem and test files, mounted like a
# live volume would be on the node.
make_origin() {
    local lv="$1" fstype="$2" size="$3"
    lvcreate -q -y -L "${size}" -n "${lv}" "${VG}" >/dev/null
    "mkfs.${fstype}" -q "/dev/${VG}/${lv}"
    mkdir -p "/mnt/${lv}"
    mount "/dev/${VG}/${lv}" "/mnt/${lv}"
    mkdir -p "/mnt/${lv}/subdir"
    echo "hello from ${lv}" >"/mnt/${lv}/file1.txt"
    echo "nested file in ${lv}" >"/mnt/${lv}/subdir/file2.txt"
}

# run_backup runs home-backup with configs/<name>.yaml and saves its output
# to ${WORK}/logs/<name>.log. It returns home-backup's exit code.
run_backup() {
    local name="$1" status=0
    docker run --rm --privileged \
        -v /dev:/dev \
        -v /lib/modules:/lib/modules:ro \
        -v "${SCRIPT_DIR}/configs:/configs:ro" \
        -v "${WORK}/repos:${WORK}/repos" \
        -e RESTIC_PASSWORD \
        "${IMAGE}" -config "/configs/${name}.yaml" -log-level debug \
        >"${WORK}/logs/${name}.log" 2>&1 || status=$?
    sed 's/^/    /' "${WORK}/logs/${name}.log"
    return "${status}"
}

expect_backed_up() {
    local lv="$1" files
    files=$(docker run --rm --entrypoint restic \
        -v "${WORK}/repos:${WORK}/repos" \
        -e RESTIC_PASSWORD \
        "${IMAGE}" --repo "${WORK}/repos/${lv}" ls latest) ||
        fail "cannot list Restic repo for ${lv}"
    for want in /file1.txt /subdir/file2.txt; do
        grep -q "${want}\$" <<<"${files}" || fail "${want} missing from ${lv} backup"
    done
}

expect_lv() {
    lvs "${VG}/$1" >/dev/null 2>&1 || fail "LV ${VG}/$1 is gone"
}

expect_log() {
    grep -qF "$2" "${WORK}/logs/$1.log" || fail "$1 log lacks: $2"
}

expect_no_lv() {
    if lvs "${VG}/$1" >/dev/null 2>&1; then
        fail "LV ${VG}/$1 still exists"
    fi
}

scenario_happy() {
    make_origin happy-ext4 ext4 256M
    make_origin happy-xfs xfs 512M
    run_backup happy || fail "home-backup exited non-zero"
    for lv in happy-ext4 happy-xfs; do
        expect_backed_up "${lv}"
        expect_no_lv "${lv}_backup_snapshot"
    done
}

# A Run that died mid-backup leaves its snapshot behind; the next Run
# removes it and carries on.
scenario_stale() {
    make_origin stale ext4 256M
    lvcreate -q -y -s -L 64M -n stale_backup_snapshot "${VG}/stale" >/dev/null
    run_backup stale || fail "home-backup exited non-zero"
    expect_log stale 'level=WARN msg="removed stale LVM snapshot" snapshot=/dev/hbtest/stale_backup_snapshot'
    expect_backed_up stale
    expect_no_lv stale_backup_snapshot
}

# lvremove refuses a snapshot that is still mounted, so the Run fails and
# leaves it alone.
scenario_stale_open() {
    make_origin stale-open ext4 256M
    lvcreate -q -y -s -L 64M -n stale-open_backup_snapshot "${VG}/stale-open" >/dev/null
    mkdir -p /mnt/stale-open-snapshot
    mount -o ro "/dev/${VG}/stale-open_backup_snapshot" /mnt/stale-open-snapshot
    if run_backup stale-open; then
        fail "home-backup succeeded with the stale snapshot mounted"
    fi
    expect_log stale-open 'remove stale LVM snapshot \"/dev/hbtest/stale-open_backup_snapshot\"'
    expect_lv stale-open_backup_snapshot
}

# A volume that only shares the snapshot's name is never removed.
scenario_foreign() {
    make_origin foreign ext4 256M
    lvcreate -q -y -L 64M -n foreign_backup_snapshot "${VG}" >/dev/null
    if run_backup foreign; then
        fail "home-backup succeeded with a foreign volume in the way"
    fi
    expect_log foreign 'is not a snapshot of \"foreign\"'
    expect_lv foreign_backup_snapshot
}

echo "--- Setting up volume group ${VG} on ${DISK} ---"
pvcreate -q "${DISK}" >/dev/null
vgcreate -q "${VG}" "${DISK}" >/dev/null
mkdir -p "${WORK}/repos" "${WORK}/logs"

failed=0
for scenario in happy stale stale_open foreign; do
    echo "--- Scenario: ${scenario} ---"
    # Not "|| status=$?": a condition context would disable set -e inside.
    set +e
    (
        set -e
        "scenario_${scenario}"
    )
    status=$?
    set -e
    if [ "${status}" -eq 0 ]; then
        echo "PASS: ${scenario}"
    else
        echo "FAIL: ${scenario}"
        failed=$((failed + 1))
    fi
done

if [ "${failed}" -ne 0 ]; then
    echo "=== ${failed} scenario(s) failed ==="
    exit 1
fi
echo "=== ALL SCENARIOS PASSED ==="
