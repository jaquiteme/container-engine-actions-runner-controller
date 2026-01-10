# Copyright 2024 Actions Runner Controller contributors
# Licensed under the Apache License, Version 2.0 (the "License");
# You may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# This file is based on the original Dockerfile from:
# https://github.com/actions/actions-runner-controller/blob/master/runner/actions-runner-dind-rootless.ubuntu-22.04.dockerfile

FROM ubuntu:24.04

ARG GH_RUNNER_VERSION="2.331.0"
ARG GH_RUNNER_USER_UID=1001

# Prevent interactive prompt
ENV DEBIAN_FRONTEND=noninteractive

# Install prerequisites packages for the runner
RUN apt-get update -y \
    && apt-get install -y software-properties-common \
    && add-apt-repository -y ppa:git-core/ppa \
    && apt-get update -y \
    && apt-get install -y --no-install-recommends \
    && apt-get install -y curl \
    tar \
    sudo \
    git \
    jq \
    iproute2 \
    iptables \
    ca-certificates \
    software-properties-common \
    uidmap \
    unzip \
    zip \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/*

# Download latest git-lfs version
RUN curl -s https://packagecloud.io/install/repositories/github/git-lfs/script.deb.sh | bash && \
    apt-get install -y --no-install-recommends git-lfs

WORKDIR /actions-runner

# Add runner user
RUN adduser --disabled-password --gecos "" --uid $GH_RUNNER_USER_UID gh-runner \
    && chown gh-runner:gh-runner -R /actions-runner \
    && usermod -aG sudo gh-runner \
    && echo "%sudo   ALL=(ALL:ALL) NOPASSWD:ALL" > /etc/sudoers \
    && echo "Defaults env_keep += \"DEBIAN_FRONTEND\"" >> /etc/sudoers

ENV HOME=/actions-runner

# Download github self-hosted runner code
RUN curl -o actions-runner.tar.gz -L https://github.com/actions/runner/releases/download/v${GH_RUNNER_VERSION}/actions-runner-linux-x64-${GH_RUNNER_VERSION}.tar.gz \
    && tar --no-same-owner --no-same-permissions --no-xattrs -xzf actions-runner.tar.gz \
    && rm actions-runner.tar.gz

# Install dependencies
RUN chmod +x /actions-runner/bin/installdependencies.sh && /actions-runner/bin/installdependencies.sh

COPY entrypoint.sh /entrypoint.sh

USER gh-runner

ENTRYPOINT ["/entrypoint.sh"]