package speech

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionManager WebSocket连接管理器
type ConnectionManager struct {
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*websocket.Conn),
	}
}

// AddConnection 添加连接
func (cm *ConnectionManager) AddConnection(sessionID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 如果已存在连接，先关闭旧连接
	if oldConn, exists := cm.connections[sessionID]; exists {
		oldConn.Close()
	}

	cm.connections[sessionID] = conn
}

// GetConnection 获取连接
func (cm *ConnectionManager) GetConnection(sessionID string) (*websocket.Conn, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, exists := cm.connections[sessionID]
	return conn, exists
}

// RemoveConnection 移除连接
func (cm *ConnectionManager) RemoveConnection(sessionID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, exists := cm.connections[sessionID]; exists {
		conn.Close()
		delete(cm.connections, sessionID)
	}
}

// CloseAll 关闭所有连接
func (cm *ConnectionManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for sessionID, conn := range cm.connections {
		conn.Close()
		delete(cm.connections, sessionID)
	}
}

// ConnectionPoolOptions 连接池配置选项
type ConnectionPoolOptions struct {
	MaxConnections    int           // 最大连接数
	ConnectionTimeout time.Duration // 连接超时时间
	ReadTimeout       time.Duration // 读取超时时间
	WriteTimeout      time.Duration // 写入超时时间
	PingInterval      time.Duration // Ping间隔
	MaxRetries        int           // 最大重试次数
}

// DefaultConnectionPoolOptions 默认连接池选项
func DefaultConnectionPoolOptions() *ConnectionPoolOptions {
	return &ConnectionPoolOptions{
		MaxConnections:    100,
		ConnectionTimeout: 30 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      30 * time.Second,
		PingInterval:      30 * time.Second,
		MaxRetries:        3,
	}
}

// ConnectionPool WebSocket连接池
type ConnectionPool struct {
	manager *ConnectionManager
	options *ConnectionPoolOptions
}

// NewConnectionPool 创建连接池
func NewConnectionPool(options *ConnectionPoolOptions) *ConnectionPool {
	if options == nil {
		options = DefaultConnectionPoolOptions()
	}

	return &ConnectionPool{
		manager: NewConnectionManager(),
		options: options,
	}
}

// GetManager 获取连接管理器
func (cp *ConnectionPool) GetManager() *ConnectionManager {
	return cp.manager
}

// ConnectWithRetry 带重试的连接建立
func (cp *ConnectionPool) ConnectWithRetry(ctx context.Context, url string, header map[string]string, sessionID string) (*websocket.Conn, error) {
	var lastErr error

	for i := 0; i < cp.options.MaxRetries; i++ {
		conn, err := cp.connect(ctx, url, header, sessionID)
		if err == nil {
			return conn, nil
		}

		lastErr = err

		// 如果是上下文取消，直接返回
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// 等待一段时间后重试
		retryDelay := time.Duration(i+1) * time.Second
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
		}
	}

	return nil, fmt.Errorf("failed to connect after %d retries, last error: %w", cp.options.MaxRetries, lastErr)
}

// connect 建立单次连接
func (cp *ConnectionPool) connect(ctx context.Context, url string, header map[string]string, sessionID string) (*websocket.Conn, error) {
	dialer := &websocket.Dialer{
		HandshakeTimeout: cp.options.ConnectionTimeout,
	}

	// 转换header格式
	wsHeader := make(map[string][]string)
	for k, v := range header {
		wsHeader[k] = []string{v}
	}

	conn, _, err := dialer.DialContext(ctx, url, wsHeader)
	if err != nil {
		return nil, fmt.Errorf("websocket dial failed: %w", err)
	}

	// 设置连接选项
	conn.SetReadDeadline(time.Now().Add(cp.options.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(cp.options.WriteTimeout))

	// 设置pong处理器
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(cp.options.ReadTimeout))
		return nil
	})

	// 添加到连接管理器
	cp.manager.AddConnection(sessionID, conn)

	// 启动ping循环
	go cp.pingLoop(ctx, conn, sessionID)

	return conn, nil
}

// pingLoop 定期发送ping消息
func (cp *ConnectionPool) pingLoop(ctx context.Context, conn *websocket.Conn, sessionID string) {
	ticker := time.NewTicker(cp.options.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(cp.options.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// 连接出错，从管理器中移除
				cp.manager.RemoveConnection(sessionID)
				return
			}
		}
	}
}

// Cleanup 清理连接池
func (cp *ConnectionPool) Cleanup() {
	cp.manager.CloseAll()
}

// ErrorHandler WebSocket错误处理器
type ErrorHandler struct {
	onConnectionError func(sessionID string, err error)
	onMessageError    func(sessionID string, err error)
	onProtocolError   func(sessionID string, err error)
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		onConnectionError: func(sessionID string, err error) {
			fmt.Printf("[WebSocket] Connection error for session %s: %v\n", sessionID, err)
		},
		onMessageError: func(sessionID string, err error) {
			fmt.Printf("[WebSocket] Message error for session %s: %v\n", sessionID, err)
		},
		onProtocolError: func(sessionID string, err error) {
			fmt.Printf("[WebSocket] Protocol error for session %s: %v\n", sessionID, err)
		},
	}
}

// SetConnectionErrorHandler 设置连接错误处理器
func (eh *ErrorHandler) SetConnectionErrorHandler(handler func(sessionID string, err error)) {
	eh.onConnectionError = handler
}

// SetMessageErrorHandler 设置消息错误处理器
func (eh *ErrorHandler) SetMessageErrorHandler(handler func(sessionID string, err error)) {
	eh.onMessageError = handler
}

// SetProtocolErrorHandler 设置协议错误处理器
func (eh *ErrorHandler) SetProtocolErrorHandler(handler func(sessionID string, err error)) {
	eh.onProtocolError = handler
}

// HandleConnectionError 处理连接错误
func (eh *ErrorHandler) HandleConnectionError(sessionID string, err error) {
	if eh.onConnectionError != nil {
		eh.onConnectionError(sessionID, err)
	}
}

// HandleMessageError 处理消息错误
func (eh *ErrorHandler) HandleMessageError(sessionID string, err error) {
	if eh.onMessageError != nil {
		eh.onMessageError(sessionID, err)
	}
}

// HandleProtocolError 处理协议错误
func (eh *ErrorHandler) HandleProtocolError(sessionID string, err error) {
	if eh.onProtocolError != nil {
		eh.onProtocolError(sessionID, err)
	}
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查是否为网络临时错误
	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure, websocket.CloseGoingAway) {
		return true
	}

	// 检查是否为超时错误
	// 这里可以根据实际需要添加更多的错误类型判断

	return false
}