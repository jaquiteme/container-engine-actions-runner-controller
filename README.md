# Container Engine Actions Runner Controller (CE-ARC)

A lightweight autoscaling self‑hosted GitHub runners with podman or docker

# About

CE-ARC is a lightweight solution to automatically scale and provision self-hosted GitHub Actions runners based on queued jobs. This repository provides a server written in Go that listen to GitHub workflow Webhook events, register runners to a GitHub organization or repository, and a Dockerfile to build GitHub runners containers image.

## Why this project

- I build this tool to serve my own purposes, primarly because I mainly use GiHub Saas and need a simple solution to run jobs that require a deployment or to access my home-lab infrastructure. But its also can fit small teams and individuals who has similar needs.

- But also running your self-hosted runner could reduce CI wait time by adding runners when workflows queue.

## Core components

- A server configure once and listen to GitHub webhook, check events integrity and automatically provision and register runners as needed.

## Quickstart (example)

## Configuration

⚠️ For now values can be provided only via environment variables.

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