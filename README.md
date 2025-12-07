# Container Engine Actions Runner Controller (CE-ARC)

A lightweight autoscaling self‑hosted GitHub runners with podman or docker

# About

CE-ARC is a lightweight solution to automatically scale and provision self-hosted GitHub Actions runners based on queued jobs. This repository provides a server written in Go that listen to GitHub workflow Webhook events, register runners to a GitHub organization or private repository, and scripts to build GitHub runners containers image.

## Why this project

- I developed this tool for my personal needs, mainly because I use GitHub to collaborate on private projects and require an easy way to execute jobs that involve deployments or access to my home lab infrastructure. However, it is also suitable for small teams and individuals with comparable requirements.

- Additionally, using a self-hosted runner might shorten CI wait durations by adding runners during queued workflows.

## Core components

- A server configured once to listen to GitHub webhooks, check event integrity, and automatically provision and register runners as needed.

## Quickstart (example)

## Configuration

⚠️ For now, values can be provided only via environment variables.

- Environnment variables:

| Name              | Description                           |
|-------------------|---------------------------------------|
|GH_RUNNER_REPO_PATH (required)| Your target GitHub repo path (Ex: name/project)|
|GH_RUNNER_REPO_ACCESS_TOKEN| Target repo path access token with repo scopes as mentionned in https://docs.github.com/en/actions/reference/runners/self-hosted-runners#authentication-requirements|
|GH_RUNNER_CT_IMAGE (required)| Your runner container image (Ex: localhost/gh-runner:latest, docker.io/202047/ce-arc-server:latest) |
|CT_ENGINE (optional)| podman or docker, if you want to force the container ngine to use|
|GH_WEBHOOK_SECRET (optional)| Your webhook secret when setting up the webhook on GitHub|

## TODO

- Check idle jobs on GitHub
- Collect runners logs