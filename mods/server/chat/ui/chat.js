class ChatUI {
    constructor() {
        this.config = window.llmChatConfig || {};
        this.eventSource = null;
        this.isConnected = false;
        this.currentMessageId = null;
        this.conversationHistory = [];
        this.sessionId = this.generateUniqueId(); // session ID for this chat session

        // DOM elements
        this.chatMessages = document.getElementById('chatMessages');
        this.chatInput = document.getElementById('chatInput');
        this.sendButton = document.getElementById('sendButton');
        this.connectionStatus = document.getElementById('connectionStatus');
        this.typingIndicator = document.getElementById('typingIndicator');

        //this.config.enableDebug();

        this.init();
    }

    init() {
        this.setupEventListeners();
        this.connectToSSE();
        this.updateWelcomeTime();

        // auto resize
        this.chatInput.addEventListener('input', this.autoResize.bind(this));
    }

    setupEventListeners() {
        // click send button
        this.sendButton.addEventListener('click', () => this.sendMessage());

        // keypress Enter (Shift+Enter for new line)
        this.chatInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.sendMessage();
            }
        });

        // re-connect on disconnect
        window.addEventListener('beforeunload', () => {
            if (this.eventSource) {
                this.eventSource.close();
            }
        });
    }

    autoResize() {
        this.chatInput.style.height = 'auto';
        this.chatInput.style.height = Math.min(this.chatInput.scrollHeight, 100) + 'px';
    }

    generateUniqueId() {
        // generates UUID v4 unique id
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            const r = Math.random() * 16 | 0;
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    connectToSSE() {
        this.updateConnectionStatus('connecting', 'Connecting...');

        try {
            // SSE endpoint
            const sseUrl = this.config.getFullUrl(this.config.server.sseEndpoint)

            // sessionid
            const urlWithSessionId = new URL(sseUrl);
            urlWithSessionId.searchParams.append('sessionId', this.sessionId);

            this.eventSource = new EventSource(urlWithSessionId.toString());

            this.eventSource.onopen = (event) => {
                this.logSSE('SSE connected:', urlWithSessionId.toString());
                this.isConnected = true;
                this.updateConnectionStatus('connected', 'Connected');
            };

            this.eventSource.onmessage = (event) => {
                this.logSSE('Raw SSE event data:', event.data); // Added debug log
                this.handleSSEMessage(event);
            };

            this.eventSource.onerror = (error) => {
                this.logSSE('SSE Error details:', error); // Added debug log
                this.isConnected = false;
                this.updateConnectionStatus('disconnected', 'Disconnected');

                // 설정에서 재연결 간격 가져오기
                const reconnectInterval = this.config.chat?.reconnectInterval || 5000;
                setTimeout(() => {
                    if (!this.isConnected) {
                        this.reconnect();
                    }
                }, reconnectInterval);
            };
        } catch (error) {
            console.error('SSE connection failed:', error);
            this.updateConnectionStatus('disconnected', 'Connection error');
        }
    }

    reconnect() {
        if (this.eventSource) {
            this.eventSource.close();
        }
        console.log('reconnecting...');
        this.connectToSSE();
    }

    updateConnectionStatus(status, text) {
        this.connectionStatus.className = `connection-status status-${status}`;
        this.connectionStatus.textContent = text;
    }

    async sendMessage() {
        const message = this.chatInput.value.trim();
        if (!message || !this.isConnected) return;

        // user message
        this.addMessage('user', message);
        this.chatInput.value = '';
        this.autoResize();

        // de-activate send button and show typing indicator
        this.sendButton.disabled = true;
        this.showTypingIndicator();

        try {
            // message endpoint
            const messageUrl = this.config.getFullUrl(this.config.server.messageEndpoint)

            this.log('Send:', messageUrl, message);

            const response = await fetch(messageUrl, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    method: 'tools/call',
                    params: {
                        name: 'chat',
                        arguments: {
                            message: message,
                            history: this.conversationHistory,
                            sessionId: this.sessionId
                        }
                    }
                })
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            // chat response is expected to be handled via SSE
        } catch (error) {
            console.error('Send message error:', error);
            this.hideTypingIndicator();
            this.addErrorMessage('Failed to send message: ' + error.message);
        } finally {
            this.sendButton.disabled = false;
        }
    }

    handleSSEMessage(event) {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'response' && data.content) {
                this.hideTypingIndicator();
                this.addMessage('assistant', data.content);
            } else if (data.type === 'stream' ) {
                if (data.seq === 0) {
                    this.addMessage('assistant', '');
                }
                if (data.delta) {
                    this.updateStreamingMessage(data.delta);
                }
                if (data.end) {
                    this.hideTypingIndicator();
                }
            } else if (data.type === 'error') {
                this.hideTypingIndicator();
                this.addErrorMessage(data.message || '알 수 없는 오류가 발생했습니다.');
            }
        } catch (error) {
            console.error('SSE 메시지 파싱 오류:', error);
        }
    }

    addMessage(sender, content) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${sender}`;

        const time = new Date().toLocaleTimeString('ko-KR', {
            hour: '2-digit',
            minute: '2-digit'
        });
        messageDiv.innerHTML = `
            <div class="message-avatar">
                ${sender === 'user' ? '👤' : '🤖'}
            </div>
            <div class="message-content">${this.formatMessage(content)}<div class="message-time">${time}</div></div>
        `;

        this.chatMessages.appendChild(messageDiv);
        this.scrollToBottom();

        // 대화 기록에 추가
        this.conversationHistory.push({
            role: sender === 'user' ? 'user' : 'assistant',
            content: content
        });

        // 대화 기록이 너무 길어지면 오래된 것들 제거
        const maxHistory = this.config.chat?.maxHistoryLength || 20;
        if (this.conversationHistory.length > maxHistory) {
            this.conversationHistory = this.conversationHistory.slice(-maxHistory);
        }
    }

    log(...args) {
        if (this.config.debug?.enableChatLog) {
            console.log('[Chat]', ...args);
        }
    }

    logSSE(...args) {
        if (this.config.debug?.showSSEMessages) {
            console.log('[SSE]', ...args);
        }
    }

    addErrorMessage(message) {
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        this.chatMessages.appendChild(errorDiv);
        this.scrollToBottom();
    }

    formatMessage(content) {
        // simple markdown-like formatting
        return content
            .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
            .replace(/\*(.*?)\*/g, '<em>$1</em>')
            .replace(/`(.*?)`/g, '<code>$1</code>')
            .replace(/\n/g, '<br>');
    }

    isTypingIndicatorVisible() {
        return this.typingIndicator.classList.contains('show');
    }
    showTypingIndicator() {
        this.typingIndicator.classList.add('show');
        this.scrollToBottom();
    }

    hideTypingIndicator() {
        this.typingIndicator.classList.remove('show');
    }

    updateStreamingMessage(delta) {
        // update streaming message (update the end of the last message)
        const messages = this.chatMessages.querySelectorAll('.message.assistant');
        if (messages.length > 0) {
            const lastMessage = messages[messages.length - 1];
            const contentDiv = lastMessage.querySelector('.message-content');
            const timeDiv = contentDiv.querySelector('.message-time');
            const currentContent = contentDiv.innerHTML.replace(timeDiv.outerHTML, '');
            contentDiv.innerHTML = this.formatMessage(currentContent + delta) + timeDiv.outerHTML;
        }
        this.scrollToBottom();
    }

    scrollToBottom() {
        this.chatMessages.scrollTop = this.chatMessages.scrollHeight;
    }

    updateWelcomeTime() {
        const welcomeTime = document.getElementById('welcomeTime');
        const welcomeMessage = document.getElementById('welcomeMessage');

        if (welcomeTime) {
            const time = new Date().toLocaleTimeString('ko-KR', {
                hour: '2-digit',
                minute: '2-digit'
            });
            welcomeTime.textContent = time;
        }

        if (welcomeMessage && this.config.ui?.welcomeMessage) {
            const timeHtml = welcomeTime ? welcomeTime.outerHTML : '';
            welcomeMessage.innerHTML = this.config.ui.welcomeMessage + timeHtml;
        }
    }
}

// Initialize chat UI when the page is fully loaded
document.addEventListener('DOMContentLoaded', () => {
    window.llmChatUI = new ChatUI();
});

// Helper functions for chat UI
window.chatUtils = {
    clearHistory: () => {
        if (window.llmChatUI) {
            window.llmChatUI.conversationHistory = [];
        }
    },

    getHistory: () => {
        if (window.llmChatUI) {
            return window.llmChatUI.conversationHistory;
        }
        return [];
    },

    reconnect: () => {
        if (window.llmChatUI) {
            window.llmChatUI.reconnect();
        }
    }
};
