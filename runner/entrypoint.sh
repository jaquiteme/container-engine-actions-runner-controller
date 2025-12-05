#!/bin/bash
set -e

if [ -z "${GH_RUNNER_REPO_PATH}" ]; then
  echo "[Error]: seems like GH_RUNNER_REPO_PATH env var is empty. This value is required"
  exit 1
fi

if [ -z "${GH_RUNNER_TOKEN}" ]; then
  echo "[Error]: seems like GH_RUNNER_TOKEN env var is empty. This value is required"
  exit 1
fi

GH_RUNNER_REPO="https://github.com/${GH_RUNNER_REPO_PATH}"

echo "Registering to projet at ${GH_RUNNER_REPO}..."

./config.sh \
  --url "${GH_RUNNER_REPO}" \
  --token "${GH_RUNNER_TOKEN}" \
  --ephemeral \
  --unattended \
  --replace

echo "Starting self hosted runner..."

./run.sh
