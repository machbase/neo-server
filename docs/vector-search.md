# Neo에서 VECTOR·검색·JSH 사용하기 (#4211)

이 기능은 #4211 Standard 엔진과 VECTOR 지원 `neo-client/v2`를 연결한다. Neo는 인덱스 알고리즘을 다시 구현하지 않는다. `CREATE INDEX`와 `VECTOR_SEARCH`·`TEXT_SEARCH`·`HYBRID_SEARCH`는 엔진 SQL이며, Neo는 VECTOR 값을 전달하고 결과를 표시한다.

## 문장에서 검색 결과까지

문장 → 임베딩 모델 → FLOAT32 숫자 벡터 → TRANSACTION 테이블의 `VECTOR(n)` → HNSW 또는 IVF 검색 → 원본 문장과 출처 ID 순서다. 문서를 저장할 때와 질문할 때 같은 모델·전처리·`EMBEDDING_SPACE`를 써야 한다. Neo의 기본 임베딩은 BGE-M3 ONNX(1024차원)이며, 외부 OpenAI 호환 임베딩 API도 설정할 수 있다. 모델은 답변을 생성하지 않는다. [JSH 예제](examples/vector-rag-jsh.js)는 검색한 근거로 LLM용 프롬프트를 만들고 종료한다.

## 준비와 실행

Linux x86-64에서 #4211 엔진을 링크한 Neo 실행 파일을 사용한다. 개발 빌드에서는 [링크 스크립트](../scripts/link-nfx-4211-engine.sh)가 `~/work/nfx`의 archive와 헤더를 연결하고 SHA-256을 출력한다. 이 브랜치는 `neo-client/v2`의 VECTOR 커밋 `3c1c286`을 사용한다. Neo의 SQLite와 엔진이 같은 FTS5 기능을 쓰도록 빌드 태그가 필요하다.

```bash
cd ~/work/neo-server
scripts/link-nfx-4211-engine.sh ~/work/nfx
go build -tags sqlite_fts5 -o tmp/machbase-neo ./cmd/machbase-neo
tmp/machbase-neo serve --data "$PWD/tmp/vector-demo-db" \
  --pref "$PWD/tmp/vector-demo-pref" \
  --mach-port 25167 --shell-port 25162 --http-port 25164 --mqtt-port 25163
```

위 포트는 충돌하지 않는 예시다. 실제 사용 환경에서는 사용 가능한 포트를 선택한다. 기존 DB와 분리된 새 `--data` 디렉터리에서 실습한다. `/db/embed`는 첫 BGE-M3 요청 때 고정 revision의 ONNX 모델·토크나이저·ONNX Runtime을 내려받아 SHA-256을 확인하고 캐시에 둔다. 첫 요청에는 모델 다운로드·로딩 시간이 든다. 서버 시작은 다운로드를 기다리지 않는다. 캐시는 `--embedding-cache`로 지정할 수 있고, 오프라인에서는 같은 SHA의 파일을 미리 배치할 수 있다. 모델 파일은 Git에 포함하지 않는다.

## SQL과 VECTOR 타입

```sql
CREATE TRANSACTION TABLE DOCS (
  ID INTEGER PRIMARY KEY,
  BODY TEXT,
  V VECTOR(1024) PROPERTY(EMBEDDING_SPACE='bge-m3-5617a9f6-onnx-l2-v1')
);
CREATE INDEX DOCS_T ON DOCS(BODY) INDEX_TYPE BM25;
CREATE INDEX DOCS_V ON DOCS(V) INDEX_TYPE IVF DISTANCE=COSINE,LISTS=2,PROBES=2;
ALTER INDEX DOCS_T WAIT TIMEOUT 120;
ALTER INDEX DOCS_V WAIT TIMEOUT 120;
```

`CREATE INDEX`는 백그라운드 구축을 접수한다. `WAIT`가 끝나고 `V$VECTOR_INDEXES`에서 `READY`를 확인한 뒤 검색한다. HNSW가 필요하면 별도 실습 테이블에 `INDEX_TYPE HNSW DISTANCE=COSINE`을 사용한다. IVF의 `PROBES`는 `ALTER INDEX DOCS_V SET PROBES=...`로 바꿀 수 있다. `MODE EXACT`는 원본에서 정확한 Top-K를 계산하고 `MODE AUTO`는 준비된 ANN 인덱스 또는 적절한 Exact 경로를 선택한다.

검색 관계와 스칼라 함수의 전체 인자·오류 계약은 엔진의 [#4211 함수 참조](https://github.com/machbase/dbms-nfx/blob/4211-vector-column/prompt/issues/4211-vector-column/manual/11_function_reference.md), CREATE/ALTER 문법은 [SQL 문법 참조](https://github.com/machbase/dbms-nfx/blob/4211-vector-column/prompt/issues/4211-vector-column/manual/15_sql_syntax_reference.md)를 따른다.

```sql
SELECT R.ID,R.BODY,R.__SEARCH_VECTOR_DISTANCE
FROM VECTOR_SEARCH(TABLE DOCS,VECTOR V,
  QUERY_VECTOR ?,
  SPACE 'bge-m3-5617a9f6-onnx-l2-v1',METRIC COSINE,MODE AUTO,TOP_K 3) R
ORDER BY R.__SEARCH_VECTOR_DISTANCE,R.ID;
```

질의의 `?`에는 JSH `db.vector()` 또는 REST의 숫자 배열 파라미터를 bind한다. `TO_VECTOR(JSON문자열,n)`도 사용할 수 있지만 문장을 임베딩하지는 않는다. `V$VECTOR_INDEXES`, `V$TEXT_INDEXES`, `V$VECTOR_IVF_MEMORY`와 `EXPLAIN FULL`로 상태·실행 경로를 확인한다. Neo `/db/query`의 `EXPLAIN FULL SELECT ...` 응답은 `PLAN` 문자열 열이다.

## REST 입력·조회·임베딩

`POST /db/embed`의 `input`은 문자열 또는 최대 32개의 문자열 배열이다. `provider`를 생략하면 내부 `bge-m3`다.
내부 모델의 기본 입력 상한은 문장당 1024토큰이며 `--embedding-max-tokens`로 조정할 수 있다. 상한을 넘으면 내용을 조용히 자르지 않고 오류를 반환한다.

```json
{"input":"펌프 밸브를 점검합니다."}
```

응답의 `data[0].vector`는 1024개 숫자, `dimensions`는 1024, `space`는 위 BGE-M3 space다. 외부 서비스는 Neo 시작 시 `--embedding-external-url`(OpenAI 호환 `/v1/embeddings` 주소), `--embedding-external-model`, `--embedding-external-dimension`, `--embedding-external-space`를 지정한다. 토큰은 `--embedding-external-key-env`가 가리키는 환경변수에 둔다. 호출 때 `"provider":"external"`을 전달한다. 외부 벡터의 차원·값·space가 맞지 않으면 오류로 처리한다.

`POST /db/query`의 `p` 안에 있는 숫자 배열은 VECTOR bind다. 위치·이름 있는 파라미터 모두 허용한다. 다음은 독립적인 3차원 타입 확인 예제다.

```sql
CREATE TRANSACTION TABLE DEMO3 (ID INTEGER PRIMARY KEY,V VECTOR(3));
```

```json
{"q":"INSERT INTO DEMO3 VALUES(?,?)","p":[1,[1,0,0]]}
```

`DOCS.V`에는 위 `/db/embed`가 반환한 **1024개** 숫자를 넣는다. `/db/query`의 VECTOR 조회는 `types`에 `"vector"`, `rows`에 숫자 배열을 반환한다. `/db/write`의 JSON·NDJSON도 VECTOR 컬럼에 숫자 배열을 받으며 CSV에서는 하나의 필드에 숫자 JSON 배열 문자열을 넣는다. MQTT 질의·쓰기와 TQL·내보내기도 같은 값 형식을 사용한다. 빈 배열, 비숫자·비유한값, 컬럼 차원 불일치는 거부한다. 현재 REST 질의의 중첩 숫자 배열은 VECTOR 의미이므로 SQL ARRAY bind가 필요할 때는 별도 명시적 표현을 사용해야 한다.

## JSH 예제 실행

Neo 서버가 실행 중인 상태에서 다른 터미널에서 실행한다. JSH CLI는 로그인한 세션으로 `new db.Client().connect()`를 사용할 수 있다. 아래 예제는 문서 여섯 개를 기본 BGE-M3로 임베딩해 BM25·IVF를 만들고, 질문의 벡터·텍스트·Hybrid 결과와 근거 프롬프트를 출력한 뒤 실습 테이블을 삭제한다.

```bash
cd ~/work/neo-server
export NEO_EMBED_CACHE="$PWD/tmp/vector-demo-pref/models"
tmp/machbase-neo shell -server 127.0.0.1:25164 -user sys -password "$NEO_PASSWORD" \
  -C @docs/examples/vector-rag-jsh.js
```

`VECTOR_INDEX=HNSW`를 JSH 환경으로 전달하면 HNSW를 사용한다. 외부 임베딩 모드는 Neo의 외부 제공자 설정과 동일한 URL·모델·차원·space를 JSH 프로세스에도 환경변수 `NEO_EMBED_EXTERNAL_URL`, `NEO_EMBED_EXTERNAL_MODEL`, `NEO_EMBED_EXTERNAL_DIMENSION`, `NEO_EMBED_EXTERNAL_SPACE`로 지정하고 `-e VECTOR_PROVIDER=external`을 추가한다. 내부와 외부 모델의 벡터는 섞지 않는다.

검증한 여섯 문장 예제에서는 IVF·HNSW 모두 `VECTOR_PLAN=ANN_DIRECT`였고, 기본 BGE-M3 질문의 VECTOR·BM25·Hybrid 상위 ID는 각각 `1,5,2`였다. 이 작은 예제의 순위는 일반적인 검색 정확도·성능 수치가 아니다.

JSH에서 숫자 배열은 `db.vector([0.1, 0.2, ...])`로 명시적으로 VECTOR에 bind한다. 조회한 VECTOR는 JavaScript 숫자 배열로 반환된다. `conn.explain(sql, true)`는 실행 계획 문자열을 돌려준다. 일반 JavaScript 배열의 다른 용도는 바뀌지 않는다.
