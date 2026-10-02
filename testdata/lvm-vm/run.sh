#!/bin/bash
# Runs the LVM regression scenarios in throwaway Ubuntu cloud VMs under
# libvirt, one VM per release, in parallel. Needs KVM, libvirt and Docker on
# this host; the user must be in the libvirt and docker groups.
#
# Usage: run.sh [--release 24.04|26.04]... [--keep] [--cleanup]
#   --release  test only this release; repeatable (default: 24.04 and 26.04)
#   --keep     keep a failed VM running and print how to reach it
#   --cleanup  remove leftover test VMs and volumes, then exit
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

export LIBVIRT_DEFAULT_URI=qemu:///system
POOL=images
NETWORK=default
PREFIX=home-backup-test
IMAGE_NAME=home-backup:lvm-vm-test

RELEASES=()
KEEP=false
CLEANUP=false
while [ $# -gt 0 ]; do
    case "$1" in
    --release)
        RELEASES+=("$2")
        shift 2
        ;;
    --keep)
        KEEP=true
        shift
        ;;
    --cleanup)
        CLEANUP=true
        shift
        ;;
    *)
        sed -n '6,9s/^# \{0,1\}//p' "$0" >&2
        exit 2
        ;;
    esac
done
if [ ${#RELEASES[@]} -eq 0 ]; then
    RELEASES=(24.04 26.04)
fi

log() { echo "=== $* ==="; }

destroy_vm() {
    local name="$1"
    virsh destroy "${name}" >/dev/null 2>&1 || true
    virsh undefine "${name}" --remove-all-storage >/dev/null 2>&1 || true
    for vol in "${name}-root.qcow2" "${name}-data.qcow2"; do
        virsh vol-delete --pool "${POOL}" "${vol}" >/dev/null 2>&1 || true
    done
}

if ${CLEANUP}; then
    for name in $(virsh list --all --name | grep "^${PREFIX}-" || true); do
        echo "Removing VM ${name}"
        destroy_vm "${name}"
    done
    for vol in $(virsh vol-list --pool "${POOL}" | awk 'NR > 2 { print $1 }' | grep "^${PREFIX}-" || true); do
        echo "Removing volume ${vol}"
        virsh vol-delete --pool "${POOL}" "${vol}" >/dev/null
    done
    exit 0
fi

WORKDIR="$(mktemp -d -t home-backup-lvm-vm.XXXXXX)"
RUN_ID="$(od -An -N3 -tx1 /dev/urandom | tr -d ' \n')"
SSH_OPTS=(
    -i "${WORKDIR}/id_ed25519"
    -o StrictHostKeyChecking=no
    -o UserKnownHostsFile=/dev/null
    -o LogLevel=ERROR
    -o ConnectTimeout=5
)

vm_name() { echo "${PREFIX}-${1//./}-${RUN_ID}"; }

teardown() {
    local kept=false
    for pid in $(jobs -p); do
        kill "${pid}" 2>/dev/null || true
    done
    wait 2>/dev/null || true
    for release in "${RELEASES[@]}"; do
        local name
        name="$(vm_name "${release}")"
        if ${KEEP} && [ -e "${WORKDIR}/${name}.failed" ]; then
            kept=true
            echo "Kept ${name}: ssh ${SSH_OPTS[*]} ubuntu@$(cat "${WORKDIR}/${name}.ip" 2>/dev/null || echo '<ip>')"
            continue
        fi
        destroy_vm "${name}"
    done
    if ${kept}; then
        echo "Remove kept VMs with: $0 --cleanup && rm -rf ${WORKDIR}"
    else
        rm -rf "${WORKDIR}"
    fi
}
trap teardown EXIT
trap 'exit 130' INT TERM

# ensure_base_image caches the release's cloud image as a pool volume,
# checked against Ubuntu's SHA256SUMS. --cleanup leaves it in place.
ensure_base_image() {
    local release="$1"
    local vol="home-backup-base-ubuntu-${release}.img"
    if virsh vol-info --pool "${POOL}" "${vol}" >/dev/null 2>&1; then
        return
    fi
    local url="https://cloud-images.ubuntu.com/releases/${release}/release"
    local file="ubuntu-${release}-server-cloudimg-amd64.img"
    local dir="${WORKDIR}/base-${release}"
    mkdir -p "${dir}"
    echo "Downloading ${file}"
    curl -fsSL -o "${dir}/${file}" "${url}/${file}"
    curl -fsSL "${url}/SHA256SUMS" | grep -F " *${file}" >"${dir}/SHA256SUMS"
    (cd "${dir}" && sha256sum --quiet -c SHA256SUMS)
    virsh vol-create-as "${POOL}" "${vol}" "$(stat -c %s "${dir}/${file}")" --format raw >/dev/null
    virsh vol-upload --pool "${POOL}" "${vol}" "${dir}/${file}" >/dev/null
    virsh pool-refresh "${POOL}" >/dev/null
    rm -rf "${dir}"
}

wait_for_ip() {
    local name="$1" ip
    for _ in $(seq 90); do
        ip="$(virsh domifaddr "${name}" --source lease | awk '/ipv4/ { sub(/\/.*/, "", $4); print $4; exit }')"
        if [ -n "${ip}" ]; then
            echo "${ip}"
            return
        fi
        sleep 2
    done
    echo "no IP address for ${name}" >&2
    return 1
}

wait_for_ssh() {
    local ip="$1"
    for _ in $(seq 60); do
        if ssh "${SSH_OPTS[@]}" "ubuntu@${ip}" true 2>/dev/null; then
            return
        fi
        sleep 2
    done
    echo "no SSH on ${ip}" >&2
    return 1
}

test_release() {
    local release="$1" name ip status
    name="$(vm_name "${release}")"
    ensure_base_image "${release}"

    echo "Creating VM ${name}"
    local base
    base="$(virsh vol-path --pool "${POOL}" "home-backup-base-ubuntu-${release}.img")"
    virsh vol-create-as "${POOL}" "${name}-root.qcow2" 20G --format qcow2 \
        --backing-vol "${base}" --backing-vol-format qcow2 >/dev/null
    virsh vol-create-as "${POOL}" "${name}-data.qcow2" 4G --format qcow2 >/dev/null
    # This host's osinfo-db has no ubuntu26.04 entry; 24.04 defaults fit both.
    virt-install --name "${name}" --memory 3072 --vcpus 2 --import \
        --osinfo ubuntu24.04 \
        --disk "vol=${POOL}/${name}-root.qcow2,bus=virtio" \
        --disk "vol=${POOL}/${name}-data.qcow2,bus=virtio" \
        --network "network=${NETWORK},model=virtio" \
        --graphics none --noautoconsole \
        --cloud-init "user-data=${WORKDIR}/user-data.yaml" >/dev/null

    ip="$(wait_for_ip "${name}")"
    echo "${ip}" >"${WORKDIR}/${name}.ip"
    wait_for_ssh "${ip}"
    echo "Waiting for cloud-init on ${ip}"
    # Exit code 2 means cloud-init finished with recoverable warnings.
    status=0
    ssh "${SSH_OPTS[@]}" "ubuntu@${ip}" sudo cloud-init status --wait >/dev/null || status=$?
    if [ "${status}" -ne 0 ] && [ "${status}" -ne 2 ]; then
        echo "cloud-init failed with exit code ${status}" >&2
        return 1
    fi

    echo "Loading image"
    ssh "${SSH_OPTS[@]}" "ubuntu@${ip}" sudo docker load -q <"${WORKDIR}/image.tar"
    scp -q -r "${SSH_OPTS[@]}" "${SCRIPT_DIR}" "ubuntu@${ip}:lvm-vm"
    ssh "${SSH_OPTS[@]}" "ubuntu@${ip}" sudo bash lvm-vm/scenarios.sh "${IMAGE_NAME}"
}

ssh-keygen -q -t ed25519 -N "" -C "${PREFIX}-${RUN_ID}" -f "${WORKDIR}/id_ed25519"
sed "s|@SSH_PUBLIC_KEY@|$(cat "${WORKDIR}/id_ed25519.pub")|" \
    "${SCRIPT_DIR}/user-data.yaml" >"${WORKDIR}/user-data.yaml"

log "Building Docker image"
docker build -q -t "${IMAGE_NAME}" "${PROJECT_ROOT}"
docker save -o "${WORKDIR}/image.tar" "${IMAGE_NAME}"

log "Testing releases: ${RELEASES[*]}"
declare -A PIDS
for release in "${RELEASES[@]}"; do
    name="$(vm_name "${release}")"
    # Not "|| touch": a condition context would disable set -e inside.
    (
        set +e
        (
            set -e
            test_release "${release}"
        )
        if [ $? -ne 0 ]; then
            touch "${WORKDIR}/${name}.failed"
        fi
    ) 2>&1 | sed -u "s/^/[${release}] /" &
    PIDS[${release}]=$!
done
wait "${PIDS[@]}"

failed=()
for release in "${RELEASES[@]}"; do
    if [ -e "${WORKDIR}/$(vm_name "${release}").failed" ]; then
        failed+=("${release}")
    fi
done
if [ ${#failed[@]} -ne 0 ]; then
    log "FAILED: ${failed[*]}"
    exit 1
fi
log "ALL RELEASES PASSED"
