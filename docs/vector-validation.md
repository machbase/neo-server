# #4211 Neo VECTOR 구현 검증 기록

검증일: 2026-10-06. Neo 브랜치 `nfx-4211-vector-support`, Standard Linux x86-64. 독립 runtime `tmp/4211-runtime`을 사용했고 기존 운영 DB는 사용하지 않았다.

## 소스·빌드 대응

| 구성 | 검증한 값 |
|---|---|
| 엔진 소스 | `~/work/nfx`, 브랜치 `4211-vector-column`, 검증 시 HEAD `623bbf5fe40dc3b3b71ae59a9e03a86cb3dbfb08` |
| 링크한 정적 archive | `machbase_home/lib/libmachengine.a`, SHA-256 `711aae01a485b25d5079e92be0fc6cf59cb9f8120a7d88f62526f8b534e83c7d` |
| Neo에 반영한 C 헤더 | `spi/mach/native/machEngine.h`, SHA-256 `3b714dcfbbb5887e80e14d8de175fb4cd58943912d14a3e19379d60f16d95811` |
| Go 클라이언트 | `neo-client/v2` VECTOR 커밋 `3c1c286`; Neo `go.mod`도 해당 pseudo-version 사용 |
| Neo 실행 파일 | `go run mage.go machbase-neo` 성공, `tmp/machbase-neo`, SHA-256 `727e82c0c80596dd76234894b3130354e77c010312592ef41c92fcaba13b1f52` |

링크 대상 archive는 [스크립트](../scripts/link-nfx-4211-engine.sh)로 지정했으며, 최종 실행 파일에 `MachBindParam`·`MachColumnDataVector` 심볼이 있음을 확인했다. `sqlite_fts5` 태그가 없는 Neo 빌드에서는 새 엔진의 `qrdFtsRegister`가 실패하므로 Mage 빌드·테스트에 태그를 적용했다. 모델 파일과 런타임 DB는 Git에 추가하지 않았다.

## 자동 테스트

| 검증 | 결과 |
|---|---|
| `go test -tags sqlite_fts5 ./mods/server -count=1` | PASS, 376.203초. 벡터 변경 뒤 추가한 MQTT VECTOR 쓰기는 후속 표적 테스트에서 별도 PASS |
| 임베딩·JSH 임베딩·JSH DB·임베디드 엔진·JSON/NDJSON/CSV·TQL 선택 패키지 전체 | PASS. 로그 `tmp/4211-selected-suite-final.log` |
| `go test ./mods/codec/... -count=1` | PASS, 코덱 하위 패키지 전체 |
| 서버 VECTOR 계획·MQTT 질의/쓰기 표적 테스트 | PASS, `tmp/4211-server-new-tests.log` |
| 임베디드 엔진 VECTOR bind/조회/NULL·동적 식/TRANSACTION APPEND·17,000차원 | PASS, `tmp/4211-vector-native-test.log` |
| TQL VECTOR APPEND | PASS, `tmp/4211-tql-vector-test.log` |
| `neo-client`의 `TestVector.*Integration`을 Neo 엔진 포트에 연결 | PASS, `tmp/4211-neoclient-integration.log` |
| 임베딩 패키지의 Windows amd64·macOS arm64 외부 API 경로 교차 컴파일 | PASS; 내장 BGE-M3 실행 지원은 Linux x86-64 |

첫 전체 선택 회귀에서 JSON decoder의 기존 단일 행 형식, REST/MQTT의 기존 복합 bind 오류 문구, MQTT APPEND 공개 시점, 포트 1 접속 오류 문구 차이가 드러났다. 단일 행 JSON 계약을 보존하고, 숫자 배열만 VECTOR로 받아들이며, MQTT APPEND 테스트는 성공 응답 이후 실제 행 공개를 기다리도록 했다. 포트 1 테스트는 연결 거부와 dial timeout을 모두 실패 경로로 인정하되 연결 실패 자체는 계속 검증한다. 이후 전체 서버 회귀와 관련 표적 테스트가 통과했다.

## 최종 `machbase-neo` 실기동

최종 실행 파일을 포트 25167(엔진)·25164(HTTP)·25163(MQTT)·25162(셸), `tmp/4211-runtime`에서 기동했다. 생성된 DB와 모델 cache는 서로 분리했다.

| 기능 | 확인 결과 |
|---|---|
| `/db/query` VECTOR | `types=["int32","vector"]`, 숫자 배열 왕복, IVF 검색 ID 1→2 및 거리 0→1 |
| `/db/write` JSON·NDJSON·CSV·APPEND | 네 형식에서 VECTOR 입력 후 ID·FLOAT32 값 일치. APPEND는 성공 응답 후 공개 완료를 기다려 확인 |
| `/db/query` `EXPLAIN FULL` | `PLAN` 열에 `VECTOR SEARCH` 실행 경로 반환 |
| `/web/api/query` | 로그인 토큰으로 VECTOR 타입·숫자 배열 반환 확인 |
| `/db/tql` | VECTOR 숫자 배열 출력 및 `SCRIPT`→`APPEND`→원본 조회 확인 |
| MQTT | VECTOR JSON 쓰기 후 조회, 동일 클라이언트 reply를 사용하는 VECTOR 검색 회귀 통과 |
| 인덱스 수명 | IVF `READY`·PROBES 변경·REBUILD, HNSW/IVF `ANN_DIRECT`, 선택 필터, UPDATE/DELETE 후 검색 반영 |
| 재시작·백업·복원 | Neo 종료·재시작 뒤 VECTOR 행·IVF `READY`·검색 결과 유지. 백업 MOUNT 조회 후 `machbase-neo restore`로 별도 runtime에 물리 복원해 VECTOR 행·IVF `READY`·검색 ID 1 재확인 |
| JSH RAG | [실행 예제](examples/vector-rag-jsh.js): 기본 BGE-M3에서 VECTOR·BM25·Hybrid의 상위 ID 1,5,2. 외부 API 모드 및 HNSW도 실행, 두 인덱스 모두 `VECTOR_PLAN=ANN_DIRECT` |
| 기본 내부 임베딩 | 첫 요청에 고정 SHA의 BGE-M3 ONNX·토크나이저·ONNX Runtime 설치, 1024차원·L2 norm 1.00000003. 기존 Go 튜토리얼 추론과 **최대 원소 차이 0** |
| 외부 임베딩 | OpenAI 호환 모의 HTTP 서버를 Neo REST와 JSH에서 호출, 지정 모델·space·3차원 벡터 일치 |

원시 실행 로그·JSON은 `tmp/4211-*`에 있다. BGE-M3 자산은 `tmp/4211-pref/models`에만 있고, `model.onnx_data`는 2,266,820,608바이트다. 첫 사용 다운로드·모델 로딩 시간과 이후 검색 지연은 별도 계측 대상이며, 이 여섯 문장 실습을 대량 데이터 정확도·성능 시험으로 해석하지 않는다. 외부 LLM의 생성 답변은 구현·검증 범위가 아니다.
