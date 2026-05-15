# Template — do not edit manually.
# Run deploy/quadlet/setup.sh to generate ce-arc-server-pod.yml.
# Placeholders substituted at generation time:
#   __HOST_UID__  → $(id -u)   UID that owns the rootless Podman socket
#   __HOST_GID__  → $(id -g)   primary GID of the same user
apiVersion: v1
kind: Pod
metadata:
  name: ce-arc-server
  labels:
    app: ce-arc-server
spec:
  # Restart policy is delegated to systemd (Restart=on-failure in the Quadlet).
  restartPolicy: Never

  containers:
    - name: ce-arc-server
      image: ce-arc-server:latest
      # Image is built locally — never pulled from a remote registry.
      imagePullPolicy: Never

      securityContext:
        # UID/GID match the host user thanks to --userns=keep-id (set in the Quadlet).
        # This ensures the container process can access the Podman socket on the host.
        runAsUser: __HOST_UID__
        runAsGroup: __HOST_GID__
        runAsNonRoot: true
        capabilities:
          drop: ["ALL"]   # The orchestrator only speaks HTTP over a Unix socket.
        readOnlyRootFilesystem: true

      # All environment variables come from the ConfigMap and the Secret.
      # No plaintext secrets in this file.
      envFrom:
        - configMapRef:
            name: ce-arc-server-config
        - secretRef:
            name: ce-arc-server-secret

      volumeMounts:
        - name: podman-socket
          mountPath: /run/podman/podman.sock
          # readOnly would prevent API writes (container create/start/remove).
          readOnly: false

  volumes:
    - name: podman-socket
      hostPath:
        # Rootless Podman socket owned by UID __HOST_UID__ on the host.
        # Substituted by setup.sh from $(id -u).
        path: /run/user/__HOST_UID__/podman/podman.sock
        type: Socket
