#!/usr/bin/env bash

# Description: A script for building a GitHub runner image
# Author : jaquiteme

set -e

IMAGE_NAME="${1:-gh-runner}"
IMAGE_VERSION="${2:-latest}"
# We will prefer using podman as its support rootless by default
CT_RUNTIME="${3:-podman}"

# Build a container image using podman
function build_podman_image() {
  local image_name="$1"
  local container_file="$2"
  podman build  -t "${image_name}" -f "${container_file}"
}

echo "====== Build Runner Image ======"

if command -v podman &> /dev/null && test "${CT_RUNTIME}" = "podman"; then
    echo "=> Using Podman"

    build_podman_image "${IMAGE_NAME}:${IMAGE_VERSION}" "runner/runner-podman-ubuntu.containerfile"
    podman image list --filter reference="${IMAGE_NAME}"
elif command -v docker &> /dev/null && test "${CT_RUNTIME}" == "docker"; then
    echo "=> Using Docker"
    sudo docker build -t "${IMAGE_NAME}" -f runner/Dockerfile
    sudo docker image list --filter reference="${IMAGE_NAME}"
else
    echo "No container engine found. Please consider installing podman or docker"
    exit 1
fi