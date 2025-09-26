package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SendSSEChunk 发送Server-Sent Events数据块
func SendSSEChunk(w http.ResponseWriter, flusher http.Flusher, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal sse payload: %v", err)
		return
	}

	if _, err := w.Write([]byte("data: ")); err != nil {
		log.Printf("failed to write sse prefix: %v", err)
		return
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("failed to write sse payload: %v", err)
		return
	}
	if _, err := w.Write([]byte("\n\n")); err != nil {
		log.Printf("failed to write sse terminator: %v", err)
		return
	}
	flusher.Flush()
}

// SetupSSEHeaders 设置Server-Sent Events响应头
func SetupSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// SendSSEEvent 发送带事件类型的SSE消息
func SendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal sse event data: %v", err)
		return
	}

	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
	flusher.Flush()
}