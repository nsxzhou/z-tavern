package speech

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// ProtocolVersion WebSocket二进制协议版本
const ProtocolVersion = 0b0001

// MessageType 消息类型
type MessageType uint8

const (
	// FullClientRequest 包含请求参数的完整客户端请求
	FullClientRequest MessageType = 0b0001
	// AudioOnlyRequest 只包含音频数据的请求
	AudioOnlyRequest MessageType = 0b0010
	// FullServerResponse 服务端返回的完整响应
	FullServerResponse MessageType = 0b1001
	// AudioOnlyServerResponse 只包含音频数据的服务端响应
	AudioOnlyServerResponse MessageType = 0b1011
	// ErrorMessage 服务端错误消息
	ErrorMessage MessageType = 0b1111
)

// MessageFlags 消息特定标志
type MessageFlags uint8

const (
	// NoSequenceNumber header后4个字节不为sequence number
	NoSequenceNumber MessageFlags = 0b0000
	// PositiveSequenceNumber header后4个字节为正数sequence number
	PositiveSequenceNumber MessageFlags = 0b0001
	// LastPacketNoSequence 最后一包，header后4个字节不为sequence number
	LastPacketNoSequence MessageFlags = 0b0010
	// NegativeSequenceNumber header后4个字节为负数sequence number（最后一包）
	NegativeSequenceNumber MessageFlags = 0b0011
	// WithEvent 表示消息携带事件元数据
	WithEvent MessageFlags = 0b0100
)

// EventType 表示服务端返回的事件类型
type EventType int32

const (
	EventTypeNone               EventType = 0
	EventTypeStartConnection    EventType = 1
	EventTypeFinishConnection   EventType = 2
	EventTypeConnectionStarted  EventType = 50
	EventTypeConnectionFailed   EventType = 51
	EventTypeConnectionFinished EventType = 52
	EventTypeSessionStarted     EventType = 150
	EventTypeSessionFinished    EventType = 152
	EventTypeSessionFailed      EventType = 153
)

// SerializationMethod 序列化方法
type SerializationMethod uint8

const (
	// NoSerialization 无序列化
	NoSerialization SerializationMethod = 0b0000
	// JSONSerialization JSON序列化
	JSONSerialization SerializationMethod = 0b0001
	// CustomSerialization 自定义序列化
	CustomSerialization SerializationMethod = 0b1111
)

// CompressionMethod 压缩方法
type CompressionMethod uint8

const (
	// NoCompression 无压缩
	NoCompression CompressionMethod = 0b0000
	// GzipCompression Gzip压缩
	GzipCompression CompressionMethod = 0b0001
	// CustomCompression 自定义压缩
	CustomCompression CompressionMethod = 0b1111
)

// Header WebSocket消息头
type Header struct {
	ProtocolVersion     uint8               // 4 bits
	HeaderSize          uint8               // 4 bits
	MessageType         MessageType         // 4 bits
	MessageFlags        MessageFlags        // 4 bits
	SerializationMethod SerializationMethod // 4 bits
	CompressionMethod   CompressionMethod   // 4 bits
	Reserved            uint8               // 8 bits
}

// Message WebSocket消息
type Message struct {
	Header      Header
	Sequence    int32 // 可选，取决于MessageFlags
	EventType   EventType
	SessionID   string
	ConnectID   string
	ErrorCode   uint32
	PayloadSize uint32
	Payload     []byte
}

// NewHeader 创建新的消息头
func NewHeader(msgType MessageType, flags MessageFlags, serialization SerializationMethod, compression CompressionMethod) Header {
	return Header{
		ProtocolVersion:     ProtocolVersion,
		HeaderSize:          0b0001, // 4字节头
		MessageType:         msgType,
		MessageFlags:        flags,
		SerializationMethod: serialization,
		CompressionMethod:   compression,
		Reserved:            0x00,
	}
}

// Encode 编码消息头为4字节
func (h *Header) Encode() []byte {
	buf := make([]byte, 4)

	// 第一个字节: protocol version (4 bits) + header size (4 bits)
	buf[0] = (h.ProtocolVersion << 4) | h.HeaderSize

	// 第二个字节: message type (4 bits) + message flags (4 bits)
	buf[1] = (uint8(h.MessageType) << 4) | uint8(h.MessageFlags)

	// 第三个字节: serialization method (4 bits) + compression method (4 bits)
	buf[2] = (uint8(h.SerializationMethod) << 4) | uint8(h.CompressionMethod)

	// 第四个字节: reserved
	buf[3] = h.Reserved

	return buf
}

// DecodeHeader 从4字节解码消息头
func DecodeHeader(data []byte) (*Header, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("header data too short: got %d, need 4", len(data))
	}

	header := &Header{
		ProtocolVersion:     (data[0] >> 4) & 0x0F,
		HeaderSize:          data[0] & 0x0F,
		MessageType:         MessageType((data[1] >> 4) & 0x0F),
		MessageFlags:        MessageFlags(data[1] & 0x0F),
		SerializationMethod: SerializationMethod((data[2] >> 4) & 0x0F),
		CompressionMethod:   CompressionMethod(data[2] & 0x0F),
		Reserved:            data[3],
	}

	if header.ProtocolVersion != ProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version: %d", header.ProtocolVersion)
	}

	return header, nil
}

// EncodeMessage 编码完整消息
func EncodeMessage(msg *Message) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	// 编码header
	headerBytes := msg.Header.Encode()
	buf.Write(headerBytes)

	// 如果需要sequence number，写入4字节sequence
	switch msg.Header.MessageFlags & 0b0011 {
	case PositiveSequenceNumber, NegativeSequenceNumber:
		seqBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(seqBytes, uint32(msg.Sequence))
		buf.Write(seqBytes)
	}

	// 如果包含事件，写入事件元数据
	if msg.Header.MessageFlags&WithEvent == WithEvent {
		eventBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(eventBytes, uint32(msg.EventType))
		buf.Write(eventBytes)

		if !eventSkipsSessionID(msg.EventType) {
			session := []byte(msg.SessionID)
			sizeBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(sizeBytes, uint32(len(session)))
			buf.Write(sizeBytes)
			if len(session) > 0 {
				buf.Write(session)
			}
		}

		if eventHasConnectID(msg.EventType) {
			connect := []byte(msg.ConnectID)
			sizeBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(sizeBytes, uint32(len(connect)))
			buf.Write(sizeBytes)
			if len(connect) > 0 {
				buf.Write(connect)
			}
		}
	}

	// 写入payload size（大端序）
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, msg.PayloadSize)
	buf.Write(sizeBytes)

	// 写入payload
	if len(msg.Payload) > 0 {
		buf.Write(msg.Payload)
	}

	return buf.Bytes(), nil
}

// DecodeMessage 解码完整消息
func DecodeMessage(reader io.Reader) (*Message, error) {
	// 读取4字节header
	headerBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, headerBytes); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	header, err := DecodeHeader(headerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	msg := &Message{Header: *header}

	// 处理可选header扩展（按字节数补齐）
	extraHeaderBytes := int(header.HeaderSize)*4 - 4
	if extraHeaderBytes > 0 {
		extra := make([]byte, extraHeaderBytes)
		if _, err := io.ReadFull(reader, extra); err != nil {
			return nil, fmt.Errorf("failed to read extended header: %w", err)
		}
	}

	// 如果需要sequence number，读取4字节
	switch header.MessageFlags & 0b0011 {
	case PositiveSequenceNumber, NegativeSequenceNumber:
		seqBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, seqBytes); err != nil {
			return nil, fmt.Errorf("failed to read sequence: %w", err)
		}
		msg.Sequence = int32(binary.BigEndian.Uint32(seqBytes))
	}

	// 如果包含事件，读取事件元数据
	if header.MessageFlags&WithEvent == WithEvent {
		var eventRaw int32
		if err := binary.Read(reader, binary.BigEndian, &eventRaw); err != nil {
			return nil, fmt.Errorf("failed to read event type: %w", err)
		}
		msg.EventType = EventType(eventRaw)

		if !eventSkipsSessionID(msg.EventType) {
			var size uint32
			if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
				return nil, fmt.Errorf("failed to read session id size: %w", err)
			}
			if size > 0 {
				session := make([]byte, size)
				if _, err := io.ReadFull(reader, session); err != nil {
					return nil, fmt.Errorf("failed to read session id: %w", err)
				}
				msg.SessionID = string(session)
			}
		}

		if eventHasConnectID(msg.EventType) {
			var size uint32
			if err := binary.Read(reader, binary.BigEndian, &size); err != nil {
				return nil, fmt.Errorf("failed to read connect id size: %w", err)
			}
			if size > 0 {
				connect := make([]byte, size)
				if _, err := io.ReadFull(reader, connect); err != nil {
					return nil, fmt.Errorf("failed to read connect id: %w", err)
				}
				msg.ConnectID = string(connect)
			}
		}
	}

	// 根据消息类型读取payload元信息
	switch header.MessageType {
	case ErrorMessage:
		codeBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, codeBytes); err != nil {
			return nil, fmt.Errorf("failed to read error code: %w", err)
		}
		msg.ErrorCode = binary.BigEndian.Uint32(codeBytes)

		sizeBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, sizeBytes); err != nil {
			return nil, fmt.Errorf("failed to read error payload size: %w", err)
		}
		msg.PayloadSize = binary.BigEndian.Uint32(sizeBytes)

	default:
		sizeBytes := make([]byte, 4)
		if _, err := io.ReadFull(reader, sizeBytes); err != nil {
			return nil, fmt.Errorf("failed to read payload size: %w", err)
		}
		msg.PayloadSize = binary.BigEndian.Uint32(sizeBytes)
	}

	// 读取payload
	if msg.PayloadSize > 0 {
		msg.Payload = make([]byte, msg.PayloadSize)
		if _, err := io.ReadFull(reader, msg.Payload); err != nil {
			return nil, fmt.Errorf("failed to read payload (expected %d bytes): %w", msg.PayloadSize, err)
		}
	}

	return msg, nil
}

// CreateFullClientRequest 创建完整客户端请求消息
func CreateFullClientRequest(payload []byte, compression CompressionMethod) *Message {
	header := NewHeader(FullClientRequest, NoSequenceNumber, JSONSerialization, compression)
	return &Message{
		Header:      header,
		PayloadSize: uint32(len(payload)),
		Payload:     payload,
	}
}

// CreateAudioOnlyRequest 创建音频请求消息
func CreateAudioOnlyRequest(audioData []byte, sequence int32, isLast bool, compression CompressionMethod) *Message {
	var flags MessageFlags
	if isLast {
		if sequence != 0 {
			flags = NegativeSequenceNumber
			sequence = -sequence // 负数表示最后一包
		} else {
			flags = LastPacketNoSequence
		}
	} else {
		if sequence > 0 {
			flags = PositiveSequenceNumber
		} else {
			flags = NoSequenceNumber
		}
	}

	header := NewHeader(AudioOnlyRequest, flags, NoSerialization, compression)
	return &Message{
		Header:      header,
		Sequence:    sequence,
		PayloadSize: uint32(len(audioData)),
		Payload:     audioData,
	}
}

// IsLastPacket 判断是否为最后一包
func eventSkipsSessionID(event EventType) bool {
	switch event {
	case EventTypeStartConnection, EventTypeFinishConnection,
		EventTypeConnectionStarted, EventTypeConnectionFailed,
		EventTypeConnectionFinished:
		return true
	default:
		return false
	}
}

func eventHasConnectID(event EventType) bool {
	switch event {
	case EventTypeConnectionStarted, EventTypeConnectionFailed, EventTypeConnectionFinished:
		return true
	default:
		return false
	}
}

// IsLastPacket 判断是否为最后一包
func (m *Message) IsLastPacket() bool {
	switch m.Header.MessageFlags & 0b0011 {
	case LastPacketNoSequence, NegativeSequenceNumber:
		return true
	default:
		return false
	}
}

// IsErrorMessage 判断是否为错误消息
func (m *Message) IsErrorMessage() bool {
	return m.Header.MessageType == ErrorMessage
}
