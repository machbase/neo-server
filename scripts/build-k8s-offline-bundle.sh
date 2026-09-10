#!/usr/bin/env bash
# Build an offline (air-gapped) deployment bundle for machbase-neo on Kubernetes.
#
# It pulls the official machbase/machbase-neo image from Docker Hub, saves it as a
# tar archive, and generates the Kubernetes manifests + README needed to bring the
# image and Pod definitions into a closed network (폐쇄망).
#
# Usage:
#   ./build-k8s-offline-bundle.sh [options]
#
# Options (env vars can also be used):
#   -s, --source-image   Source image repo   (default: machbase/machbase-neo)
#   -t, --tag            Image tag           (default: v8.7.0)
#   -p, --platform       Target platform     (default: linux/amd64)
#   -r, --registry       Target private registry host[:port] used in generated
#                        manifests, e.g. registry.closed-network.local:5000
#                        (default: registry.closed-network.local:5000)
#   -n, --namespace       Kubernetes namespace (default: machbase-neo)
#   -o, --out-dir         Output directory     (default: <script-dir>/tmp/k8s)
set -euo pipefail

SOURCE_IMAGE="machbase/machbase-neo"
IMAGE_TAG="v8.7.0"
PLATFORM="linux/amd64"
TARGET_REGISTRY="registry.closed-network.local:5000"
NAMESPACE="machbase-neo"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${SCRIPT_DIR}/tmp/k8s"

while [[ $# -gt 0 ]]; do
    case "$1" in
        -s|--source-image) SOURCE_IMAGE="$2"; shift 2 ;;
        -t|--tag) IMAGE_TAG="$2"; shift 2 ;;
        -p|--platform) PLATFORM="$2"; shift 2 ;;
        -r|--registry) TARGET_REGISTRY="$2"; shift 2 ;;
        -n|--namespace) NAMESPACE="$2"; shift 2 ;;
        -o|--out-dir) OUT_DIR="$2"; shift 2 ;;
        -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *) echo "Unknown option: $1" >&2; exit 1 ;;
    esac
done

IMAGE_REF="${SOURCE_IMAGE}:${IMAGE_TAG}"
IMAGE_FILE_BASENAME="machbase-neo-${IMAGE_TAG}"
TARGET_IMAGE_REF="${TARGET_REGISTRY}/${SOURCE_IMAGE}:${IMAGE_TAG}"

IMAGES_DIR="${OUT_DIR}/images"
MANIFESTS_DIR="${OUT_DIR}/manifests"

echo "==> Output directory : ${OUT_DIR}"
echo "==> Source image      : ${IMAGE_REF} (${PLATFORM})"
echo "==> Target registry   : ${TARGET_REGISTRY}"
echo "==> Namespace          : ${NAMESPACE}"

mkdir -p "${IMAGES_DIR}" "${MANIFESTS_DIR}"

echo "==> Pulling ${IMAGE_REF} for platform ${PLATFORM} ..."
docker pull --platform "${PLATFORM}" "${IMAGE_REF}"

TAR_PATH="${IMAGES_DIR}/${IMAGE_FILE_BASENAME}.tar"
echo "==> Saving image to ${TAR_PATH} ..."
docker save "${IMAGE_REF}" -o "${TAR_PATH}"

echo "==> Computing sha256 checksum ..."
( cd "${IMAGES_DIR}" && sha256sum "$(basename "${TAR_PATH}")" > "$(basename "${TAR_PATH}").sha256" )

echo "==> Generating load-image.sh (run inside the closed network) ..."
cat > "${IMAGES_DIR}/load-image.sh" <<EOF
#!/usr/bin/env bash
# Load the offline image tar into a container runtime and push it to the
# closed-network private registry. Run this script INSIDE the closed network,
# on a host that has docker (or containerd) and access to the private registry.
set -euo pipefail
cd "\$(dirname "\${BASH_SOURCE[0]}")"

TAR_FILE="${IMAGE_FILE_BASENAME}.tar"
SHA_FILE="\${TAR_FILE}.sha256"
SOURCE_TAG="${IMAGE_REF}"
TARGET_TAG="${TARGET_IMAGE_REF}"

echo "==> Verifying checksum ..."
sha256sum -c "\${SHA_FILE}"

echo "==> Loading \${TAR_FILE} into local docker ..."
docker load -i "\${TAR_FILE}"

echo "==> Tagging \${SOURCE_TAG} -> \${TARGET_TAG} ..."
docker tag "\${SOURCE_TAG}" "\${TARGET_TAG}"

echo "==> Pushing \${TARGET_TAG} to the private registry ..."
docker push "\${TARGET_TAG}"

echo "==> Done. Image available at \${TARGET_TAG}"
EOF
chmod +x "${IMAGES_DIR}/load-image.sh"

echo "==> Generating Kubernetes manifests ..."

cat > "${MANIFESTS_DIR}/00-namespace.yaml" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
EOF

cat > "${MANIFESTS_DIR}/01-service.yaml" <<EOF
apiVersion: v1
kind: Service
metadata:
  name: machbase-neo
  namespace: ${NAMESPACE}
  labels:
    app: machbase-neo
spec:
  clusterIP: None
  selector:
    app: machbase-neo
  ports:
    - name: shell
      port: 5652
    - name: mqtt
      port: 5653
    - name: http
      port: 5654
    - name: grpc
      port: 5655
    - name: mach
      port: 5656
EOF

cat > "${MANIFESTS_DIR}/02-statefulset.yaml" <<EOF
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: machbase-neo
  namespace: ${NAMESPACE}
spec:
  serviceName: machbase-neo
  replicas: 1
  selector:
    matchLabels:
      app: machbase-neo
  template:
    metadata:
      labels:
        app: machbase-neo
    spec:
      # Uncomment if the private registry requires authentication.
      # imagePullSecrets:
      #   - name: regcred
      securityContext:
        fsGroup: 1000
      containers:
        - name: machbase-neo
          image: ${TARGET_IMAGE_REF}
          imagePullPolicy: IfNotPresent
          # Overrides the image's default CMD; change values per instance as needed.
          args:
            - "--host=0.0.0.0"
            - "--data=/data"
            - "--file=/file"
            - "--backup-dir=/backups"
            - "--http-port=5654"
            - "--log-level=INFO"
          ports:
            - containerPort: 5652
              name: shell
            - containerPort: 5653
              name: mqtt
            - containerPort: 5654
              name: http
            - containerPort: 5655
              name: grpc
            - containerPort: 5656
              name: mach
          volumeMounts:
            - name: data
              mountPath: /data
            - name: file
              mountPath: /file
            - name: backups
              mountPath: /backups
          readinessProbe:
            tcpSocket:
              port: 5654
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: 5654
            initialDelaySeconds: 30
            periodSeconds: 20
          resources:
            requests:
              cpu: "500m"
              memory: "1Gi"
            limits:
              cpu: "2"
              memory: "4Gi"
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 50Gi
    - metadata:
        name: file
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 20Gi
    - metadata:
        name: backups
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 50Gi
EOF

echo "==> Rendering README.md from template ..."
OUT_DIR_BASENAME="$(basename "${OUT_DIR}")"
export IMAGE_TAG IMAGE_REF PLATFORM TARGET_REGISTRY TARGET_IMAGE_REF IMAGE_FILE_BASENAME NAMESPACE OUT_DIR_BASENAME
envsubst < "${SCRIPT_DIR}/build-k8s-offline-bundle.md" > "${OUT_DIR}/README.md"

echo ""
echo "==> Done. Bundle generated at: ${OUT_DIR}"
find "${OUT_DIR}" -type f | sort
