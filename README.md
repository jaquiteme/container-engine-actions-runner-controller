# Self‑hosted Podman & Actions Runner Controller (SPARC)

A lightweight autoscaling self‑hosted runners with podman

# About

SPARC is a lightweight solution to automatically scale and provision self-hosted GitHub Actions runners based on queued jobs for podman. This repository provides a server written in Go that listen to GitHub workflow Webhook events, register runners to a GitHub organization or repository, and a Dockerfile to build GitHub runners containers image.

## Why this project

- I build this tool to serve my own purposes, primarly because I mainly use GiHub Saas and need a simple solution to run jobs that require a deployment or to access my home-lab infrastructure. But its also can fit small teams and individuals who has similar needs.

- But also running your self-hosted runner could reduce CI wait time by adding runners when workflows queue.

## Core components

- A server configure once and listen to GitHub webhook, check events integrity and automatically provision and register runners as needed.

## Quickstart (example)

## Configuration

- For now values can be provided only via environment variables.
- Env Variables:
    - GH_RUNNER_REPO_PATH (required)
    - GH_RUNNER_TOKEN (required)
    - GH_RUNNER_CT_IMAGE (required)
    - CT_RUNTIME (optional)
    -  GH_WEBHOOK_SECRET (optional)

## TODO

- Write a Makefile to automate installation
- Check idle jobs on GitHub
- Collect runners logs