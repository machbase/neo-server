// Ollama Chat UI 설정
window.llmChatConfig = {
    // 서버 설정
    server: {
        baseUrl: 'http://127.0.0.1:5654',
        chatPath: '/db/chat',
        sseEndpoint: '/sse',
        messageEndpoint: '/message',
        modelsEndpoint: '/models'
    },
    
    // 채팅 설정
    chat: {
        maxHistoryLength: 20,           // 최대 대화 기록 수
        reconnectInterval: 5000,        // 재연결 시도 간격 (ms)
        typingIndicatorDelay: 100,      // 타이핑 인디케이터 표시 지연 (ms)
        autoScrollDelay: 100            // 자동 스크롤 지연 (ms)
    },
    
    // UI 설정
    ui: {
        theme: 'default',               // 테마 (향후 확장용)
        showTimestamps: true,           // 메시지 시간 표시
        showConnectionStatus: true,     // 연결 상태 표시
        enableMarkdown: true,           // 마크다운 지원
        maxInputHeight: 100,            // 입력창 최대 높이 (px)
        welcomeMessage: 'Hello! This is the Machbase Assistant. How can I help you?'
        },
    
    // 디버그 설정
    debug: {
        enableChatLog: false,         // 콘솔 로그 활성화
        showSSEMessages: false,          // SSE 메시지 로그 표시
        enablePerformanceMetrics: false // 성능 메트릭 수집
    }
};

// 설정 유틸리티 함수들
window.llmChatConfig.getFullUrl = function(endpoint) {
    const { server } = this;
    return `${server.baseUrl}${server.chatPath}${endpoint}`;
};

window.llmChatConfig.updateServer = function(baseUrl, port) {
    this.server.baseUrl = `${baseUrl}:${port}`;
};

window.llmChatConfig.enableDebug = function() {
    this.debug.enableChatLog = true;
    this.debug.showSSEMessages = true;
    this.debug.enablePerformanceMetrics = true;
    console.log('Debug mode enabled.');
};

window.llmChatConfig.disableDebug = function() {
    this.debug.enableChatLog = false;
    this.debug.showSSEMessages = false;
    this.debug.enablePerformanceMetrics = false;
    console.log('Debug mode disabled.');
};
