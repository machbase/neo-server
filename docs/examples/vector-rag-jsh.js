'use strict';

// Run in machbase-neo shell. The model creates embeddings; Machbase stores and searches them.
(async () => {
    const process = require('process');
    const embedding = require('@jsh/embedding');
    const db = require('@jsh/db');
    const provider = process.env.get('VECTOR_PROVIDER') || undefined;
    const indexKind = (process.env.get('VECTOR_INDEX') || 'IVF').toUpperCase();
    const question = process.env.get('VECTOR_QUESTION') || '펌프 밸브 누수를 어떻게 점검하나요?';
    if (indexKind !== 'IVF' && indexKind !== 'HNSW') throw new Error('VECTOR_INDEX must be IVF or HNSW');

    const documents = [
        [1, '펌프 밸브를 점검하고 누수를 확인합니다.'],
        [2, '펌프 진동과 소음의 원인을 분석합니다.'],
        [3, '모터 베어링을 교체하는 절차입니다.'],
        [4, '데이터베이스 백업 파일을 복원합니다.'],
        [5, '밸브 누수 시 배관 연결부를 점검합니다.'],
        [6, '서버 접속 오류가 발생하면 포트와 인증 정보를 확인합니다.']
    ];
    const options = provider ? { provider } : undefined;
    const vectors = await embedding.embedMany(documents.map(doc => doc[1]), options);
    const dimension = vectors[0].dimension;
    const space = vectors[0].space;
    if (!Number.isInteger(dimension) || dimension < 1 || dimension > 65536 ||
        vectors.length !== documents.length ||
        vectors.some(item => item.dimension !== dimension || item.space !== space)) {
        throw new Error('embedding metadata mismatch');
    }
    const sqlSpace = space.replace(/'/g, "''");
    const conn = new db.Client().connect();
    try {
        conn.exec('CREATE TRANSACTION TABLE NEO_JSH_RAG(' +
                  'ID INTEGER PRIMARY KEY,BODY TEXT,V VECTOR(' + dimension + ')' +
                  " PROPERTY(EMBEDDING_SPACE='" + sqlSpace + "'))");
        try {
            for (let i = 0; i < documents.length; i++) {
                conn.exec('INSERT INTO NEO_JSH_RAG VALUES(?,?,?)', documents[i][0],
                          documents[i][1], db.vector(vectors[i].vector));
            }
            conn.exec('CREATE INDEX NEO_JSH_RAG_T ON NEO_JSH_RAG(BODY) INDEX_TYPE BM25');
            conn.exec('ALTER INDEX NEO_JSH_RAG_T WAIT TIMEOUT 120');
            const vectorOptions = indexKind === 'IVF'
                ? 'IVF DISTANCE=COSINE,LISTS=2,PROBES=2'
                : 'HNSW DISTANCE=COSINE';
            conn.exec('CREATE INDEX NEO_JSH_RAG_V ON NEO_JSH_RAG(V) INDEX_TYPE ' + vectorOptions);
            conn.exec('ALTER INDEX NEO_JSH_RAG_V WAIT TIMEOUT 120');

            const query = await embedding.embed(question, options);
            if (query.dimension !== dimension || query.space !== space) {
                throw new Error('question embedding space differs from document space');
            }
            const vectorSQL = 'SELECT R.ID,R.BODY FROM ' +
                'VECTOR_SEARCH(TABLE NEO_JSH_RAG,VECTOR V,QUERY_VECTOR ?,' +
                "SPACE '" + sqlSpace + "',METRIC COSINE,MODE AUTO,TOP_K 3) R " +
                'ORDER BY R.__SEARCH_VECTOR_DISTANCE,R.ID';
            const vectorIDs = [];
            for (const row of conn.query(vectorSQL, db.vector(query.vector))) vectorIDs.push(row.ID);
            const explainSQL = vectorSQL.replace('QUERY_VECTOR ?',
                "QUERY_VECTOR TO_VECTOR('" + JSON.stringify(query.vector) + "'," + dimension + ")");
            const plan = conn.explain(explainSQL, true);
            console.println('VECTOR_PLAN=' +
                (plan.includes('ANN_DIRECT') ? 'ANN_DIRECT' :
                 plan.includes('BTREE_EXACT') ? 'BTREE_EXACT' : 'OTHER'));

            const textIDs = [];
            for (const row of conn.query(
                'SELECT R.ID FROM TEXT_SEARCH(TABLE NEO_JSH_RAG,TEXT BODY,QUERY_TEXT ?,TOP_K 3) R',
                question)) textIDs.push(row.ID);

            const hybridSQL = 'SELECT R.ID,R.BODY FROM ' +
                'HYBRID_SEARCH(TABLE NEO_JSH_RAG,VECTOR V,TEXT BODY,' +
                "QUERY_VECTOR ?,QUERY_TEXT ?,SPACE '" + sqlSpace + "'," +
                'METRIC COSINE,MODE AUTO,TOP_K 3) R ORDER BY R.__SEARCH_SCORE DESC,R.ID';
            const sources = [];
            for (const row of conn.query(hybridSQL, db.vector(query.vector), question)) {
                sources.push('[' + row.ID + '] ' + row.BODY);
            }
            if (!vectorIDs.length || !textIDs.length || !sources.length) {
                throw new Error('search returned no evidence');
            }
            console.println('VECTOR=' + JSON.stringify(vectorIDs));
            console.println('BM25=' + JSON.stringify(textIDs));
            console.println('HYBRID=' + JSON.stringify(sources));
            console.println('LLM에 전달할 근거 프롬프트:\n질문: ' + question +
                            '\n다음 근거만 사용하고 출처 ID를 표시하세요.\n' + sources.join('\n'));
        } finally {
            conn.exec('DROP TABLE NEO_JSH_RAG');
        }
    } finally {
        conn.close();
    }
})().catch(err => {
    console.println(err.message);
    require('process').exit(1);
});
