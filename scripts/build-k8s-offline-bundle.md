# machbase-neo 폐쇄망 반입 산출물 (${IMAGE_TAG})

이 디렉터리는 `build-k8s-offline-bundle.sh` 스크립트가 생성한 산출물입니다.
Docker Hub의 `${IMAGE_REF}` 이미지를 폐쇄망 내부 Kubernetes 클러스터에 배포하기 위해 필요한
이미지 아카이브, 로드 스크립트, Pod(=StatefulSet) 매니페스트를 포함합니다.

자세한 배경 설명은 [plan-build-k8s-image.md](../../../../plan-build-k8s-image.md) 문서를 참고하세요.

## 디렉터리 구조

```
${OUT_DIR_BASENAME}/
├── README.md
├── images/
│   ├── ${IMAGE_FILE_BASENAME}.tar          # docker save 로 저장한 이미지 아카이브
│   ├── ${IMAGE_FILE_BASENAME}.tar.sha256   # 무결성 검증용 체크섬
│   └── load-image.sh                        # 폐쇄망 내부에서 실행할 적재/푸시 스크립트
└── manifests/
    ├── 00-namespace.yaml
    ├── 01-service.yaml                       # Headless Service (shell/mqtt/http/grpc/mach)
    └── 02-statefulset.yaml                   # StatefulSet + PVC (data/file/backups)
```

## 1. 반입 전 확인 사항

- 소스 이미지: `${IMAGE_REF}` (platform: `${PLATFORM}`)
- 반입 대상 레지스트리: `${TARGET_REGISTRY}` (매니페스트의 `image:` 값에 이미 반영되어 있습니다)
- 다른 아키텍처(예: `linux/arm64`)나 다른 태그가 필요하면 스크립트를 옵션과 함께 다시 실행하세요.

```bash
./build-k8s-offline-bundle.sh \
  --tag ${IMAGE_TAG} \
  --platform linux/arm64 \
  --registry ${TARGET_REGISTRY}
```

## 2. 매체 반입 및 무결성 검증

1. `images/${IMAGE_FILE_BASENAME}.tar`, `images/${IMAGE_FILE_BASENAME}.tar.sha256`,
   `images/load-image.sh`, `manifests/` 디렉터리를 조직의 반입 승인 절차(매체 스캔 등)에 따라 폐쇄망으로 옮깁니다.
2. 폐쇄망 내부에서 체크섬을 재검증합니다.

```bash
cd images
sha256sum -c ${IMAGE_FILE_BASENAME}.tar.sha256
```

## 3. 이미지 적재 및 내부 레지스트리 push

`docker`/레지스트리 접근 권한이 있는 폐쇄망 내부 호스트에서 실행합니다.

```bash
cd images
./load-image.sh
```

이 스크립트는 다음을 수행합니다.

1. `docker load`로 tar를 로컬 이미지로 적재
2. `${IMAGE_REF}` → `${TARGET_IMAGE_REF}` 로 재태깅
3. 내부 프라이빗 레지스트리로 `docker push`

레지스트리가 자체 서명 인증서를 사용한다면, push 전에 해당 호스트의 docker/containerd에
`insecure-registries` 또는 `certs.d/<registry>/ca.crt` 설정을 먼저 추가하세요.

containerd만 있는 노드(k3s 등)라면 `docker load` 대신 다음을 사용할 수 있습니다.

```bash
sudo ctr -n k8s.io images import ${IMAGE_FILE_BASENAME}.tar
```

## 4. Kubernetes 배포

```bash
kubectl apply -f manifests/00-namespace.yaml
kubectl apply -f manifests/01-service.yaml
kubectl apply -f manifests/02-statefulset.yaml
```

private registry가 인증을 요구하면 `imagePullSecrets`를 생성하고
`manifests/02-statefulset.yaml`의 주석 처리된 `imagePullSecrets` 항목을 활성화하세요.

```bash
kubectl -n ${NAMESPACE} create secret docker-registry regcred \
  --docker-server=${TARGET_REGISTRY} \
  --docker-username=<user> \
  --docker-password=<password>
```

## 5. 인스턴스별 시작 옵션 변경

`manifests/02-statefulset.yaml`의 `spec.template.spec.containers[0].args`만 수정하면
`--data`, `--file`, `--backup-dir`, `--http-port`, `--log-level` 등 시작 옵션을 인스턴스마다
다르게 지정할 수 있습니다 (예: Kustomize overlay 또는 Helm values로 관리).

## 6. 배포 검증

```bash
kubectl -n ${NAMESPACE} get pods -w
kubectl -n ${NAMESPACE} logs -f machbase-neo-0
kubectl -n ${NAMESPACE} exec -it machbase-neo-0 -- \
  /opt/machbase-neo shell --server 127.0.0.1:5652 -u sys -p manager
```

## 7. PVC 및 StorageClass

`manifests/02-statefulset.yaml`의 `volumeClaimTemplates`는 `storageClassName`을 지정하지 않았으므로
클러스터의 기본 StorageClass가 사용됩니다. 특정 StorageClass를 지정하려면 각 `volumeClaimTemplates`
항목에 `storageClassName: <내부-storage-class>`를 추가하세요.
