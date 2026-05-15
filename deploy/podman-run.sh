#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# deploy/podman-run.sh — Run ce-arc-server in a rootless Podman container.
#
# Rootless Podman socket setup
# ─────────────────────────────
# The orchestrator needs to talk to the host Podman daemon to create/remove
# runner containers. The rootless socket lives at:
#   $XDG_RUNTIME_DIR/podman/podman.sock   (e.g. /run/user/1000/podman/podman.sock)
#
# Activate it once with:
#   systemctl --user enable --now podman.socket
#
# Security model
# ──────────────
# --userns=keep-id          The invoking user's UID is mapped 1:1 inside the
#                           container (e.g. host UID 1000 = container UID 1000).
# --user $(id -u):$(id -g)  Override the image's default USER so the process
#                           UID matches the socket file owner on the host.
# --cap-drop=ALL            The orchestrator only speaks HTTP over a Unix socket;
#                           it requires zero Linux capabilities.
# --security-opt=no-new-privileges Prevent setuid/setgid escalation.
# --read-only               Root filesystem is immutable; the binary makes no
#                           disk writes at runtime.
#
# Podman socket mount
# ───────────────────
# The host socket is mounted at /run/podman/podman.sock inside the container.
# That path is the first one checked by WhichContainerEngine() in main.go, so
# CT_ENGINE=podman is explicit but auto-detection would also succeed.
# The `:z` suffix applies the correct SELinux label (shared, container-readable).
# -----------------------------------------------------------------------------

set -euo pipefail

############################################################################
# Configurable defaults (override via environment)
############################################################################

IMAGE="${IMAGE:-ce-arc-server:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-ce-arc-server}"
PORT="${PORT:-3000}"

############################################################################
# Locate the rootless Podman socket
############################################################################

XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
PODMAN_SOCKET="${XDG_RUNTIME_DIR}/podman/podman.sock"

if [[ ! -S "${PODMAN_SOCKET}" ]]; then
    echo "ERROR: Podman socket not found at ${PODMAN_SOCKET}" >&2
    echo "       Enable it with: systemctl --user enable --now podman.socket" >&2
    exit 1
fi

############################################################################
# Required env vars — fail fast if missing
############################################################################

: "${GH_RUNNER_REPO_PATH:?Required: GitHub repo path (e.g. owner/repo)}"
: "${GH_RUNNER_REPO_ACCESS_TOKEN:?Required: GitHub personal access token with repo scope}"
: "${GH_RUNNER_CT_IMAGE:?Required: runner container image (e.g. localhost/gh-runner:latest)}"
: "${GH_WEBHOOK_SECRET:?Required: GitHub webhook secret}"

############################################################################
# Build optional flags
############################################################################

EXTRA_ENV=()
if [[ -n "${GH_RUNNER_LABELS:-}" ]]; then
    EXTRA_ENV+=(-e "GH_RUNNER_LABELS=${GH_RUNNER_LABELS}")
fi

############################################################################
# Run
############################################################################

podman run \
    --name "${CONTAINER_NAME}" \
    --replace \
    --detach \
    \
    --userns=keep-id \
    --user "$(id -u):$(id -g)" \
    \
    --cap-drop=ALL \
    --security-opt=no-new-privileges \
    \
    --read-only \
    \
    -v "${PODMAN_SOCKET}":/run/podman/podman.sock:z \
    \
    -e CT_ENGINE=podman \
    -e PORT="${PORT}" \
    -e GH_RUNNER_REPO_PATH="${GH_RUNNER_REPO_PATH}" \
    -e GH_RUNNER_REPO_ACCESS_TOKEN="${GH_RUNNER_REPO_ACCESS_TOKEN}" \
    -e GH_RUNNER_CT_IMAGE="${GH_RUNNER_CT_IMAGE}" \
    -e GH_WEBHOOK_SECRET="${GH_WEBHOOK_SECRET}" \
    "${EXTRA_ENV[@]}" \
    \
    -p "${PORT}:${PORT}" \
    \
    "${IMAGE}"

echo ""
echo "ce-arc-server is running. Useful commands:"
echo "  podman logs -f ${CONTAINER_NAME}"
echo "  podman stop   ${CONTAINER_NAME}"
echo "  podman rm     ${CONTAINER_NAME}"
