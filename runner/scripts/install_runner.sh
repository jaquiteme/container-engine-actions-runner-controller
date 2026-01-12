#!/usr/bin/env bash
# Description: install github action runner

source utils.sh
source logger.sh

GH_RUNNER_VERSION=$1
TARGETPLATFORM=$2

if [ -z "${GH_RUNNER_VERSION}" ]; then
    log.error "GH_RUNNER_VERSION is not provided."
    exit 1
fi

export ARCH="x64"
if [ "${TARGETPLATFORM}" = "linux/arm64" ]; then
    ARCH="arm64"
fi

log.debug "Downloading runner sources."
curl -o actions-runner.tar.gz \
    -L https://github.com/actions/runner/releases/download/v${GH_RUNNER_VERSION}/actions-runner-linux-${ARCH}-${GH_RUNNER_VERSION}.tar.gz

log.debug "Extracting runner sources downloaded"
tar --no-same-owner --no-same-permissions --no-xattrs -xzf actions-runner.tar.gz
rm actions-runner.tar.gz

log.debug "Installing runner dependencies."

chmod +x $HOME/bin/installdependencies.sh
$HOME/bin/installdependencies.sh