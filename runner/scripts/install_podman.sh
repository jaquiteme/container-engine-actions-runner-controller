#!/usr/bin/env bash
# Description: Install and configure podman

source utils.sh
source logger.sh

LOCAL_USER="$1" || true
shift

# Install podman using package manager
apt-get update && apt-get upgrade -y
apt-get install -y podman
remove_apt_cache

# Podman rootless HOME
mkdir -p /home/$LOCAL_USER/.local/share/containers
chown $LOCAL_USER:$LOCAL_USER -R /home/$LOCAL_USER/.local

# Podman container rootful configuration files
curl -L https://raw.githubusercontent.com/containers/image_build/refs/heads/main/podman/containers.conf > /etc/containers/containers.conf
if [ ! -f "/etc/containers/containers.conf" ]; then
    log.error "Podman rootful file /etc/containers/containers.conf does not exists."
    exit 1
fi

# Podman container rootless configuration files
mkdir -p /home/$LOCAL_USER/.config/containers
curl -L https://raw.githubusercontent.com/containers/image_build/refs/heads/main/podman/podman-containers.conf > /home/$LOCAL_USER/.config/containers/containers.conf
if [ ! -f "/home/$LOCAL_USER/.config/containers/containers.conf" ]; then
    log.error "Podman rootless file /home/$LOCAL_USER/.config/containers/containers.conf does not exists."
    exit 1
fi
chown $LOCAL_USER:$LOCAL_USER -R /home/$LOCAL_USER/.config

# Podman rootful storage
mkdir -p /etc/containers
printf "[storage]\ndriver=\"overlay\"\n" > /etc/containers/storage.conf

# chmod containers.conf and adjust storage.conf to enable Fuse storage.
chmod 644 /etc/containers/containers.conf; \
    sed -i -e 's|^#mount_program|mount_program|g' \
    -e '/additionalimage.*/a "/var/lib/shared",' \
    -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' /etc/containers/storage.conf
mkdir -p /var/lib/shared/overlay-images \
    /var/lib/shared/overlay-layers \
    /var/lib/shared/vfs-images \
    /var/lib/shared/vfs-layers; \
    touch /var/lib/shared/overlay-images/images.lock; \
    touch /var/lib/shared/overlay-layers/layers.lock; \
    touch /var/lib/shared/vfs-images/images.lock; \
    touch /var/lib/shared/vfs-layers/layers.lock

# Create docker alias to target podman
ln -s /usr/bin/podman /usr/local/bin/docker
