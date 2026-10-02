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

echo "--- Setting up volume group ${VG} on ${DISK} ---"
pvcreate -q "${DISK}" >/dev/null
vgcreate -q "${VG}" "${DISK}" >/dev/null
mkdir -p "${WORK}/repos" "${WORK}/logs"

failed=0
for scenario in happy; do
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
