FROM ubuntu:24.04

ARG GH_RUNNER_VERSION="2.334.0"
ARG TARGETPLATFORM
ARG GH_RUNNER_USER_UID=1001
ARG GH_RUNNER_USER_NAME="gh-runner"

# Prevent interactive prompt
ENV DEBIAN_FRONTEND=noninteractive

# Install prerequisites packages for the runner
RUN apt-get update -y \
    && apt-get install -y software-properties-common \
    && add-apt-repository -y ppa:git-core/ppa \
    && apt-get update -y \
    && apt-get install -y --no-install-recommends \
    && apt-get install -y \
    curl \
    tar \
    sudo \
    git \
    jq \
    iptables \
    ca-certificates \
    software-properties-common \
    gnupg \
    uidmap \
    fuse-overlayfs \
    unzip \
    zip \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /tmp/*

# Download latest git-lfs version
RUN curl -s https://packagecloud.io/install/repositories/github/git-lfs/script.deb.sh | bash \
    && apt-get install -y --no-install-recommends git-lfs

# Create runner user and setup sudo
RUN adduser --disabled-password --gecos "" --uid $GH_RUNNER_USER_UID $GH_RUNNER_USER_NAME \
    && usermod -aG sudo $GH_RUNNER_USER_NAME \
    && printf '%s\n' '%sudo ALL=(ALL:ALL) NOPASSWD:ALL' 'Defaults env_keep += "DEBIAN_FRONTEND"' > /etc/sudoers.d/gh-runner \
    && chmod 440 /etc/sudoers.d/gh-runner

ENV HOME=/home/$GH_RUNNER_USER_NAME

# Copy custom scripts
COPY scripts/install_runner.sh \
    scripts/logger.sh \
    scripts/utils.sh /usr/bin/

ENV _CONTAINERS_USERNS_CONFIGURED=""

WORKDIR /home/$GH_RUNNER_USER_NAME

# Runner install
RUN chmod +x /usr/bin/install_runner.sh \
    && /usr/bin/install_runner.sh ${GH_RUNNER_VERSION} ${TARGETPLATFORM}

COPY entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod +x /usr/bin/entrypoint.sh

USER $GH_RUNNER_USER_NAME

ENTRYPOINT ["/bin/bash", "-c"]
CMD [ "entrypoint.sh" ]