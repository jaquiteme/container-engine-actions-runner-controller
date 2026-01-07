#!/bin/bash
set -e

ARG="$1"
shift || true

# Config a runner
config_runner() {
   if [ -z "${GH_RUNNER_REPO_PATH}" ]; then
    echo "[Error]: seems like GH_RUNNER_REPO_PATH env var is empty. This value is required"
    exit 1
  fi

  GH_RUNNER_REPO="https://github.com/${GH_RUNNER_REPO_PATH}"

  if [ -z "${GH_RUNNER_TOKEN}" ]; then
    echo "[Error]: seems like GH_RUNNER_TOKEN env var is empty. This value is required"
    exit 1
  fi

  echo "Registering to projet at ${GH_RUNNER_REPO}..."

  ./config.sh \
    --url "${GH_RUNNER_REPO}" \
    --token "${GH_RUNNER_TOKEN}" \
    --ephemeral \
    --unattended \
    --replace
}

# Start runner
start_runner() {
  echo "Starting self hosted runner..."
  ./run.sh
}

config_runner
start_runner