package speech

import (
	"bytes"
	"testing"

	"github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// TestProtocolEncoding 测试二进制协议编解码
func TestProtocolEncoding(t *testing.T) {
	// 创建测试消息
	testPayload := []byte("test payload data")
	header := NewHeader(FullClientRequest, NoSequenceNumber, JSONSerialization, GzipCompression)

	originalMsg := &Message{
		Header:      header,
		PayloadSize: uint32(len(testPayload)),
		Payload:     testPayload,
	}

	// 编码消息
	encodedData, err := EncodeMessage(originalMsg)
	if err != nil {
		t.Fatalf("Failed to encode message: %v", err)
	}

	// 解码消息
	reader := bytes.NewReader(encodedData)
	decodedMsg, err := DecodeMessage(reader)
	if err != nil {
		t.Fatalf("Failed to decode message: %v", err)
	}

	// 验证解码结果
	if decodedMsg.Header.MessageType != originalMsg.Header.MessageType {
		t.Errorf("Message type mismatch: got %v, want %v", decodedMsg.Header.MessageType, originalMsg.Header.MessageType)
	}

	if decodedMsg.PayloadSize != originalMsg.PayloadSize {
		t.Errorf("Payload size mismatch: got %v, want %v", decodedMsg.PayloadSize, originalMsg.PayloadSize)
	}

	if !bytes.Equal(decodedMsg.Payload, originalMsg.Payload) {
		t.Errorf("Payload mismatch: got %v, want %v", decodedMsg.Payload, originalMsg.Payload)
	}
}

// TestCompressionFunctions 测试压缩功能
func TestCompressionFunctions(t *testing.T) {
	testData := []byte("This is a test string for compression testing. " +
		"It should be long enough to see the compression effect. " +
		"Repeat: This is a test string for compression testing.")

	// 测试Gzip压缩
	compressed, err := CompressPayload(testData, GzipCompression)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	// 压缩后的数据应该比原数据小（对于这个测试数据）
	if len(compressed) >= len(testData) {
		t.Logf("Warning: Compressed size (%d) >= original size (%d)", len(compressed), len(testData))
	}

	// 解压缩
	decompressed, err := DecompressPayload(compressed, GzipCompression)
	if err != nil {
		t.Fatalf("Failed to decompress data: %v", err)
	}

	// 验证解压缩结果
	if !bytes.Equal(decompressed, testData) {
		t.Errorf("Decompressed data doesn't match original")
	}
}

// TestConnectionManager 测试连接管理器
func TestConnectionManager(t *testing.T) {
	manager := NewConnectionManager()
	sessionID := "test-session-123"

	// 测试添加连接（使用nil作为占位符，实际使用时应该是真实的WebSocket连接）
	manager.AddConnection(sessionID, nil)

	// 测试获取连接
	conn, exists := manager.GetConnection(sessionID)
	if !exists {
		t.Errorf("Connection should exist for session %s", sessionID)
	}
	if conn != nil {
		t.Logf("Note: Using nil connection for testing")
	}

	// 手动从map中删除，避免调用Close()方法
	manager.mu.Lock()
	delete(manager.connections, sessionID)
	manager.mu.Unlock()

	// 验证连接已删除
	_, exists = manager.GetConnection(sessionID)
	if exists {
		t.Errorf("Connection should not exist after removal")
	}
}

// TestSpeechConfigValidation 测试语音配置验证
func TestSpeechConfigValidation(t *testing.T) {
	// 测试有效配置
	validConfig := &speech.SpeechConfig{
		AppID:       "test-app-id",
		AccessToken: "test-access-token",
		Region:      "cn-beijing",
		BaseURL:     "https://openspeech.bytedance.com",
		ASRModel:    "bigmodel",
		ASRLanguage: "zh-CN",
		TTSVoice:    "zh_male_M392_conversation_wvae_bigtts",
		TTSSpeed:    1.0,
		TTSVolume:   1.0,
		TTSLanguage: "zh-CN",
		Timeout:     30,
	}

	service := NewService(validConfig)
	if service == nil {
		t.Fatalf("Failed to create service with valid config")
	}

	if service.config.AppID != validConfig.AppID {
		t.Errorf("AppID mismatch: got %s, want %s", service.config.AppID, validConfig.AppID)
	}

	if service.config.AccessToken != validConfig.AccessToken {
		t.Errorf("AccessToken mismatch: got %s, want %s", service.config.AccessToken, validConfig.AccessToken)
	}

	// 清理
	service.Cleanup()
}

// TestTTSRequestBuilding 测试TTS请求构建
func TestTTSRequestBuilding(t *testing.T) {
	config := &speech.SpeechConfig{
		AppID:       "test-app-id",
		AccessToken: "test-access-token",
		TTSVoice:    "zh_male_M392_conversation_wvae_bigtts",
		TTSSpeed:    1.2,
		TTSVolume:   1.1,
		TTSLanguage: "zh-CN",
	}

	client := NewVolcengineTTSClient(config)

	req := &speech.TTSRequest{
		SessionID: "test-session",
		Text:      "这是一个测试文本",
		Voice:     "", // 空值，应该使用配置中的默认值
		Format:    "mp3",
		Language:  "zh-CN",
	}

	ttsReq, userUID := client.buildTTSRequest(req, "", "")

	if userUID != req.SessionID {
		t.Errorf("UID should reuse session ID: got %s, want %s", userUID, req.SessionID)
	}

	if ttsReq.User.UID != req.SessionID {
		t.Errorf("Request UID mismatch: got %s, want %s", ttsReq.User.UID, req.SessionID)
	}

	if ttsReq.ReqParams.Speaker != config.TTSVoice {
		t.Errorf("Speaker should use config default: got %s, want %s", ttsReq.ReqParams.Speaker, config.TTSVoice)
	}

	if ttsReq.ReqParams.AudioParams.Format != "mp3" {
		t.Errorf("Format fallback mismatch: got %s, want mp3", ttsReq.ReqParams.AudioParams.Format)
	}

	if ttsReq.ReqParams.AudioParams.SampleRate != 24000 {
		t.Errorf("Sample rate mismatch: got %d, want 24000", ttsReq.ReqParams.AudioParams.SampleRate)
	}

	if !ttsReq.ReqParams.AudioParams.EnableTimestamp {
		t.Errorf("EnableTimestamp should default to true")
	}

	if ttsReq.ReqParams.AudioParams.SpeedRatio != config.TTSSpeed {
		t.Errorf("Speed ratio mismatch: got %f, want %f", ttsReq.ReqParams.AudioParams.SpeedRatio, config.TTSSpeed)
	}

	if ttsReq.ReqParams.AudioParams.VolumeRatio != config.TTSVolume {
		t.Errorf("Volume ratio mismatch: got %f, want %f", ttsReq.ReqParams.AudioParams.VolumeRatio, config.TTSVolume)
	}

	if ttsReq.ReqParams.Text != req.Text {
		t.Errorf("Text mismatch: got %s, want %s", ttsReq.ReqParams.Text, req.Text)
	}

	if ttsReq.ReqParams.Language != req.Language {
		t.Errorf("Language mismatch: got %s, want %s", ttsReq.ReqParams.Language, req.Language)
	}

	expectedAdditions := "{\"disable_markdown_filter\":false}"
	if ttsReq.ReqParams.Additions != expectedAdditions {
		t.Errorf("Additions mismatch: got %s, want %s", ttsReq.ReqParams.Additions, expectedAdditions)
	}
}

// TestASRRequestBuilding 测试ASR请求构建
func TestASRRequestBuilding(t *testing.T) {
	config := &speech.SpeechConfig{
		AppID:       "test-app-id",
		AccessToken: "test-access-token",
		ASRModel:    "bigmodel",
		ASRLanguage: "zh-CN",
	}

	client := NewVolcengineASRClient(config)

	audioData := bytes.NewReader([]byte("fake audio data"))
	req := &speech.ASRRequest{
		SessionID: "test-session",
		AudioData: audioData,
		Format:    "wav",
		Language:  "", // 空值，应该使用配置中的默认值
	}

	asrReq := client.buildASRRequest(req)

	// 验证构建的请求
	if asrReq.User.UID != req.SessionID {
		t.Errorf("UID should be session ID: got %s, want %s", asrReq.User.UID, req.SessionID)
	}

	if asrReq.Audio.Format != req.Format {
		t.Errorf("Format mismatch: got %s, want %s", asrReq.Audio.Format, req.Format)
	}

	if asrReq.Audio.Language != config.ASRLanguage {
		t.Errorf("Language should use config default: got %s, want %s", asrReq.Audio.Language, config.ASRLanguage)
	}

	if asrReq.Request.ModelName != "bigmodel" {
		t.Errorf("Model name should be 'bigmodel': got %s", asrReq.Request.ModelName)
	}

	// 验证默认设置
	if !asrReq.Request.EnableITN {
		t.Errorf("EnableITN should be true by default")
	}

	if !asrReq.Request.EnablePunc {
		t.Errorf("EnablePunc should be true by default")
	}

	if !asrReq.Request.ShowUtterances {
		t.Errorf("ShowUtterances should be true by default")
	}
}

// BenchmarkProtocolEncoding 协议编解码性能基准测试
func BenchmarkProtocolEncoding(b *testing.B) {
	testPayload := make([]byte, 1024) // 1KB payload
	for i := range testPayload {
		testPayload[i] = byte(i % 256)
	}

	header := NewHeader(FullClientRequest, NoSequenceNumber, JSONSerialization, NoCompression)
	msg := &Message{
		Header:      header,
		PayloadSize: uint32(len(testPayload)),
		Payload:     testPayload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 编码
		encoded, _ := EncodeMessage(msg)

		// 解码
		reader := bytes.NewReader(encoded)
		_, _ = DecodeMessage(reader)
	}
}

// BenchmarkCompression 压缩性能基准测试
func BenchmarkCompression(b *testing.B) {
	testData := make([]byte, 8192) // 8KB data
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 压缩
		compressed, _ := CompressPayload(testData, GzipCompression)

		// 解压缩
		_, _ = DecompressPayload(compressed, GzipCompression)
	}
}
