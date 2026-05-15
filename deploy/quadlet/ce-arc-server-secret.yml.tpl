# Template — do not edit manually and do NOT commit the generated file.
# Run deploy/quadlet/setup.sh to produce ce-arc-server-secret.yml.
# Values are injected from environment variables at generation time:
#   __GH_RUNNER_REPO_ACCESS_TOKEN__  → $GH_RUNNER_REPO_ACCESS_TOKEN
#   __GH_WEBHOOK_SECRET__            → $GH_WEBHOOK_SECRET
#
# Kubernetes Secret kind — processed by Podman 4.8+ via `podman play kube`.
# stringData values are stored as-is by Podman and never exposed in logs.
apiVersion: v1
kind: Secret
metadata:
  name: ce-arc-server-secret
type: Opaque
stringData:
  # GitHub Personal Access Token with repo scope (or fine-grained: Actions — read/write).
  GH_RUNNER_REPO_ACCESS_TOKEN: "__GH_RUNNER_REPO_ACCESS_TOKEN__"

  # Shared secret configured in the GitHub repository webhook settings.
  GH_WEBHOOK_SECRET: "__GH_WEBHOOK_SECRET__"
