#!/bin/bash
# Description: runner entrypoint

set -e

source logger.sh

# Config a runner
config_runner() {
   if [ -z "${REPO_URL}" ]; then
    log.error "Seems like REPO_URL env var is empty. This value is required"
    exit 1
  fi

  if [ -z "${RUNNER_TOKEN}" ]; then
    log.error "Seems like RUNNER_TOKEN env var is empty. This value is required"
    exit 1
  fi

  log.debug "Registering runner to ${REPO_URL}..."

  retries_left=5

  while [[ ${retries_left} -gt 0 ]]; do
    ./config.sh \
      --url "${REPO_URL}" \
      --token "${RUNNER_TOKEN}" \
      --ephemeral \
      --unattended \
      --disableupdate \
      --

    if [ -f .runner ]; then
      log.debug "Runner successfully configured."
      break
    fi
      log.error "Failed to configure runner. Retrying"
      retries_left=$((retries_left - 1))
      sleep 1
  done

  if [ ! -f .runner ]; then
      log.error "Failed to configure runner."
      exit 2
  fi

}

# Start runner
start_runner() {
  echo "Starting self hosted runner..."
  ./run.sh
}

config_runner
start_runner