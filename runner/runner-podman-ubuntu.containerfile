FROM ubuntu:24.04

ARG GH_RUNNER_VERSION="2.331.0"
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

# Install podman
RUN apt-get update && apt-get install -y podman && rm -rf /var/lib/apt/lists/*

# Download latest git-lfs version
RUN curl -s https://packagecloud.io/install/repositories/github/git-lfs/script.deb.sh | bash && \
    apt-get install -y --no-install-recommends git-lfs

# Add runner user
RUN adduser --disabled-password --gecos "" --uid $GH_RUNNER_USER_UID $GH_RUNNER_USER_NAME \
    && usermod -aG sudo $GH_RUNNER_USER_NAME \
    && echo "%sudo   ALL=(ALL:ALL) NOPASSWD:ALL" > /etc/sudoers \
    && echo "Defaults env_keep += \"DEBIAN_FRONTEND\"" >> /etc/sudoers; \
    echo $GH_RUNNER_USER_NAME:1002:64535 > /etc/subuid; \
    echo $GH_RUNNER_USER_NAME:1002:64535 > /etc/subgid;

RUN mkdir -p /home/$GH_RUNNER_USER_NAME/.local/share/containers && \
    chown $GH_RUNNER_USER_NAME:$GH_RUNNER_USER_NAME -R /home/$GH_RUNNER_USER_NAME/.local
# /var/lib/containers => for podman rootful
# /home/$GH_RUNNER_USER_NAME/.local/share/containers => for podman rootless
VOLUME /var/lib/containers
VOLUME /home/$GH_RUNNER_USER_NAME/.local/share/containers

# Add
ADD https://raw.githubusercontent.com/containers/image_build/refs/heads/main/podman/containers.conf /etc/containers/containers.conf
ADD https://raw.githubusercontent.com/containers/image_build/refs/heads/main/podman/podman-containers.conf /home/$GH_RUNNER_USER_NAME/.config/containers/containers.conf

RUN chown $GH_RUNNER_USER_NAME:$GH_RUNNER_USER_NAME -R /home/$GH_RUNNER_USER_NAME

# Configure Podman rootful storage
RUN mkdir -p /etc/containers && \
    printf "[storage]\ndriver=\"overlay\"\n" > /etc/containers/storage.conf

# Configure Alias docker -> podman
RUN ln -s /usr/bin/podman /usr/local/bin/docker

ENV _CONTAINERS_USERNS_CONFIGURED=""
ENV HOME=/home/$GH_RUNNER_USER_NAME

WORKDIR /home/$GH_RUNNER_USER_NAME

# chmod containers.conf and adjust storage.conf to enable Fuse storage.
RUN chmod 644 /etc/containers/containers.conf; \
    sed -i -e 's|^#mount_program|mount_program|g' \
    -e '/additionalimage.*/a "/var/lib/shared",' \
    -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' /etc/containers/storage.conf
RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers /var/lib/shared/vfs-images /var/lib/shared/vfs-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock; touch /var/lib/shared/vfs-images/images.lock; touch /var/lib/shared/vfs-layers/layers.lock

# Download github self-hosted runner code
RUN curl -o actions-runner.tar.gz -L https://github.com/actions/runner/releases/download/v${GH_RUNNER_VERSION}/actions-runner-linux-x64-${GH_RUNNER_VERSION}.tar.gz \
    && tar --no-same-owner --no-same-permissions --no-xattrs -xzf actions-runner.tar.gz \
    && rm actions-runner.tar.gz

# Install dependencies
RUN chmod +x /home/$GH_RUNNER_USER_NAME/bin/installdependencies.sh && \
    /home/$GH_RUNNER_USER_NAME/bin/installdependencies.sh

COPY entrypoint.sh /usr/bin/entrypoint.sh
RUN chmod +x /usr/bin/entrypoint.sh

USER $GH_RUNNER_USER_NAME

ENTRYPOINT ["/bin/bash", "-c"]
CMD [ "entrypoint.sh" ]