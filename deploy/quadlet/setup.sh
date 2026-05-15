#!/usr/bin/env bash
# deploy/quadlet/setup.sh — Generate, install, and activate the ce-arc-server Quadlet.
#
# What this script does:
#   1. Substitutes __HOST_UID__ / __HOST_GID__ into the pod YAML template.
#   2. Injects secret values from environment variables into the secret YAML template.
#   3. Copies all Quadlet files to ~/.config/containers/systemd/.
#   4. Reloads systemd and starts (or restarts) the ce-arc-server service.
#
# Prerequisites:
#   • Podman >= 4.8
#   • The ce-arc-server image must already be built:
#       podman build -t ce-arc-server:latest .   (run from the repo root)
#   • The rootless Podman socket must be active:
#       systemctl --user enable --now podman.socket
#   • Edit ce-arc-server-config.yml before running (GH_RUNNER_REPO_PATH, GH_RUNNER_CT_IMAGE).
#
# Usage:
#   export GH_RUNNER_REPO_ACCESS_TOKEN="ghp_..."
#   export GH_WEBHOOK_SECRET="your-webhook-secret"
#   bash deploy/quadlet/setup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SYSTEMD_DIR="${HOME}/.config/containers/systemd"

HOST_UID="$(id -u)"
HOST_GID="$(id -g)"
SOCKET_PATH="${XDG_RUNTIME_DIR:-/run/user/${HOST_UID}}/podman/podman.sock"

############################################################################
# Pre-flight checks
############################################################################

: "${GH_RUNNER_REPO_ACCESS_TOKEN:?Export GH_RUNNER_REPO_ACCESS_TOKEN before running this script}"
: "${GH_WEBHOOK_SECRET:?Export GH_WEBHOOK_SECRET before running this script}"

if ! command -v podman &>/dev/null; then
    echo "ERROR: podman is not installed or not in PATH." >&2
    exit 1
fi

PODMAN_MINOR="$(podman --version | grep -oP '(?<=podman version )\d+\.\d+' | cut -d. -f2)"
PODMAN_MAJOR="$(podman --version | grep -oP '(?<=podman version )\d+')"
if [[ "${PODMAN_MAJOR}" -lt 4 ]] || { [[ "${PODMAN_MAJOR}" -eq 4 ]] && [[ "${PODMAN_MINOR}" -lt 8 ]]; }; then
    echo "ERROR: Podman >= 4.8 is required (found $(podman --version))." >&2
    exit 1
fi

if [[ ! -S "${SOCKET_PATH}" ]]; then
    echo "ERROR: Podman socket not found at ${SOCKET_PATH}" >&2
    echo "       Enable it with: systemctl --user enable --now podman.socket" >&2
    exit 1
fi

if ! podman image exists ce-arc-server:latest; then
    echo "ERROR: Image 'ce-arc-server:latest' not found." >&2
    echo "       Build it from the repo root with: podman build -t ce-arc-server:latest ." >&2
    exit 1
fi

############################################################################
# Generate pod YAML
############################################################################

POD_YAML="${SCRIPT_DIR}/ce-arc-server-pod.yml"
sed \
    -e "s|__HOST_UID__|${HOST_UID}|g" \
    -e "s|__HOST_GID__|${HOST_GID}|g" \
    "${SCRIPT_DIR}/ce-arc-server-pod.yml.tpl" \
    > "${POD_YAML}"
echo "[1/4] Generated ${POD_YAML} (UID=${HOST_UID} GID=${HOST_GID})"

############################################################################
# Generate secret YAML
############################################################################
# Mode 600 prevents other users from reading the plaintext values.

SECRET_YAML="${SCRIPT_DIR}/ce-arc-server-secret.yml"
sed \
    -e "s|__GH_RUNNER_REPO_ACCESS_TOKEN__|${GH_RUNNER_REPO_ACCESS_TOKEN}|g" \
    -e "s|__GH_WEBHOOK_SECRET__|${GH_WEBHOOK_SECRET}|g" \
    "${SCRIPT_DIR}/ce-arc-server-secret.yml.tpl" \
    > "${SECRET_YAML}"
chmod 600 "${SECRET_YAML}"
echo "[2/4] Generated ${SECRET_YAML} (permissions: 600)"

############################################################################
# Install Quadlet files
############################################################################

mkdir -p "${SYSTEMD_DIR}"

install -m 644 "${SCRIPT_DIR}/ce-arc-server.kube"       "${SYSTEMD_DIR}/"
install -m 644 "${SCRIPT_DIR}/ce-arc-server-pod.yml"    "${SYSTEMD_DIR}/"
install -m 644 "${SCRIPT_DIR}/ce-arc-server-config.yml" "${SYSTEMD_DIR}/"
install -m 600 "${SECRET_YAML}"                          "${SYSTEMD_DIR}/"

echo "[3/4] Installed Quadlet files to ${SYSTEMD_DIR}/"

############################################################################
# Activate the service
############################################################################

systemctl --user daemon-reload

if systemctl --user is-active --quiet ce-arc-server; then
    systemctl --user restart ce-arc-server
    echo "[4/4] Service restarted."
else
    systemctl --user enable --now ce-arc-server
    echo "[4/4] Service enabled and started."
fi

echo ""
systemctl --user status ce-arc-server --no-pager --lines=0 || true
echo ""
echo "Follow logs : journalctl --user -u ce-arc-server -f"
echo "Stop        : systemctl --user stop ce-arc-server"
echo "Uninstall   : systemctl --user disable --now ce-arc-server && rm ${SYSTEMD_DIR}/ce-arc-server.*"
