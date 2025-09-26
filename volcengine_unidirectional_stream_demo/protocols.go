package protocols

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

type (
	// EventType defines the event type which determines the event of the message.
	EventType int32
	// MsgType defines message type which determines how the message will be
	// serialized with the protocol.
	MsgType uint8
	// MsgTypeFlagBits defines the 4-bit message-type specific flags. The specific
	// values should be defined in each specific usage scenario.
	MsgTypeFlagBits uint8
	// VersionBits defines the 4-bit version type.
	VersionBits uint8
	// HeaderSizeBits defines the 4-bit header-size type.
	HeaderSizeBits uint8
	// SerializationBits defines the 4-bit serialization method type.
	SerializationBits uint8
	// CompressionBits defines the 4-bit compression method type.
	CompressionBits uint8
)

const (
	MsgTypeFlagNoSeq       MsgTypeFlagBits = 0     // Non-terminal packet with no sequence
	MsgTypeFlagPositiveSeq MsgTypeFlagBits = 0b1   // Non-terminal packet with sequence > 0
	MsgTypeFlagLastNoSeq   MsgTypeFlagBits = 0b10  // last packet with no sequence
	MsgTypeFlagNegativeSeq MsgTypeFlagBits = 0b11  // last packet with sequence < 0
	MsgTypeFlagWithEvent   MsgTypeFlagBits = 0b100 // Payload contains event number (int32)
)

const (
	Version1 VersionBits = iota + 1
	Version2
	Version3
	Version4
)

const (
	HeaderSize4 HeaderSizeBits = iota + 1
	HeaderSize8
	HeaderSize12
	HeaderSize16
)

const (
	SerializationRaw    SerializationBits = 0
	SerializationJSON   SerializationBits = 0b1
	SerializationThrift SerializationBits = 0b11
	SerializationCustom SerializationBits = 0b1111
)

const (
	CompressionNone   CompressionBits = 0
	CompressionGzip   CompressionBits = 0b1
	CompressionCustom CompressionBits = 0b1111
)

const (
	MsgTypeInvalid              MsgType = 0
	MsgTypeFullClientRequest    MsgType = 0b1
	MsgTypeAudioOnlyClient      MsgType = 0b10
	MsgTypeFullServerResponse   MsgType = 0b1001
	MsgTypeAudioOnlyServer      MsgType = 0b1011
	MsgTypeFrontEndResultServer MsgType = 0b1100
	MsgTypeError                MsgType = 0b1111

	MsgTypeServerACK = MsgTypeAudioOnlyServer
)

func (t MsgType) String() string {
	switch t {
	case MsgTypeFullClientRequest:
		return "MsgType_FullClientRequest"
	case MsgTypeAudioOnlyClient:
		return "MsgType_AudioOnlyClient"
	case MsgTypeFullServerResponse:
		return "MsgType_FullServerResponse"
	case MsgTypeAudioOnlyServer:
		return "MsgType_AudioOnlyServer" // MsgTypeServerACK
	case MsgTypeError:
		return "MsgType_Error"
	case MsgTypeFrontEndResultServer:
		return "MsgType_FrontEndResultServer"
	default:
		return fmt.Sprintf("MsgType_(%d)", t)
	}
}

const (
	// Default event, applicable for scenarios not using events or not requiring event transmission,
	// or for scenarios using events, non-zero values can be used to validate event legitimacy
	EventType_None EventType = 0
	// 1 ~ 49 for upstream Connection events
	EventType_StartConnection  EventType = 1
	EventType_StartTask        EventType = 1 // Alias of "StartConnection"
	EventType_FinishConnection EventType = 2
	EventType_FinishTask       EventType = 2 // Alias of "FinishConnection"
	// 50 ~ 99 for downstream Connection events
	// Connection established successfully
	EventType_ConnectionStarted EventType = 50
	EventType_TaskStarted       EventType = 50 // Alias of "ConnectionStarted"
	// Connection failed (possibly due to authentication failure)
	EventType_ConnectionFailed EventType = 51
	EventType_TaskFailed       EventType = 51 // Alias of "ConnectionFailed"
	// Connection ended
	EventType_ConnectionFinished EventType = 52
	EventType_TaskFinished       EventType = 52 // Alias of "ConnectionFinished"
	// 100 ~ 149 for upstream Session events
	EventType_StartSession  EventType = 100
	EventType_CancelSession EventType = 101
	EventType_FinishSession EventType = 102
	// 150 ~ 199 for downstream Session events
	EventType_SessionStarted  EventType = 150
	EventType_SessionCanceled EventType = 151
	EventType_SessionFinished EventType = 152
	EventType_SessionFailed   EventType = 153
	// Usage events
	EventType_UsageResponse EventType = 154
	EventType_ChargeData    EventType = 154 // Alias of "UsageResponse"
	// 200 ~ 249 for upstream general events
	EventType_TaskRequest  EventType = 200
	EventType_UpdateConfig EventType = 201
	// 250 ~ 299 for downstream general events
	EventType_AudioMuted EventType = 250
	// 300 ~ 349 for upstream TTS events
	EventType_SayHello EventType = 300
	// 350 ~ 399 for downstream TTS events
	EventType_TTSSentenceStart     EventType = 350
	EventType_TTSSentenceEnd       EventType = 351
	EventType_TTSResponse          EventType = 352
	EventType_TTSEnded             EventType = 359
	EventType_PodcastRoundStart    EventType = 360
	EventType_PodcastRoundResponse EventType = 361
	EventType_PodcastRoundEnd      EventType = 362
	// 450 ~ 499 for downstream ASR events
	EventType_ASRInfo     EventType = 450
	EventType_ASRResponse EventType = 451
	EventType_ASREnded    EventType = 459
	// 500 ~ 549 for upstream dialogue events
	// (Ground-Truth-Alignment) text for speech synthesis
	EventType_ChatTTSText EventType = 500
	// 550 ~ 599 for downstream dialogue events
	EventType_ChatResponse EventType = 550
	EventType_ChatEnded    EventType = 559
	// 650 ~ 699 for downstream dialogue events
	// Events for source (original) language subtitle.
	EventType_SourceSubtitleStart    EventType = 650
	EventType_SourceSubtitleResponse EventType = 651
	EventType_SourceSubtitleEnd      EventType = 652
	// Events for target (translation) language subtitle.
	EventType_TranslationSubtitleStart    EventType = 653
	EventType_TranslationSubtitleResponse EventType = 654
	EventType_TranslationSubtitleEnd      EventType = 655
)

func (t EventType) String() string {
	switch t {
	case EventType_None:
		return "EventType_None"
	case EventType_StartConnection:
		return "EventType_StartConnection"
	case EventType_FinishConnection:
		return "EventType_FinishConnection"
	case EventType_ConnectionStarted:
		return "EventType_ConnectionStarted"
	case EventType_ConnectionFailed:
		return "EventType_ConnectionFailed"
	case EventType_ConnectionFinished:
		return "EventType_ConnectionFinished"
	case EventType_StartSession:
		return "EventType_StartSession"
	case EventType_CancelSession:
		return "EventType_CancelSession"
	case EventType_FinishSession:
		return "EventType_FinishSession"
	case EventType_SessionStarted:
		return "EventType_SessionStarted"
	case EventType_SessionCanceled:
		return "EventType_SessionCanceled"
	case EventType_SessionFinished:
		return "EventType_SessionFinished"
	case EventType_SessionFailed:
		return "EventType_SessionFailed"
	case EventType_UsageResponse:
		return "EventType_UsageResponse"
	case EventType_TaskRequest:
		return "EventType_TaskRequest"
	case EventType_UpdateConfig:
		return "EventType_UpdateConfig"
	case EventType_AudioMuted:
		return "EventType_AudioMuted"
	case EventType_SayHello:
		return "EventType_SayHello"
	case EventType_TTSSentenceStart:
		return "EventType_TTSSentenceStart"
	case EventType_TTSSentenceEnd:
		return "EventType_TTSSentenceEnd"
	case EventType_TTSResponse:
		return "EventType_TTSResponse"
	case EventType_TTSEnded:
		return "EventType_TTSEnded"
	case EventType_PodcastRoundStart:
		return "EventType_PodcastRoundStart"
	case EventType_PodcastRoundResponse:
		return "EventType_PodcastRoundResponse"
	case EventType_PodcastRoundEnd:
		return "EventType_PodcastRoundEnd"
	case EventType_ASRInfo:
		return "EventType_ASRInfo"
	case EventType_ASRResponse:
		return "EventType_ASRResponse"
	case EventType_ASREnded:
		return "EventType_ASREnded"
	case EventType_ChatTTSText:
		return "EventType_ChatTTSText"
	case EventType_ChatResponse:
		return "EventType_ChatResponse"
	case EventType_ChatEnded:
		return "EventType_ChatEnded"
	case EventType_SourceSubtitleStart:
		return "EventType_SourceSubtitleStart"
	case EventType_SourceSubtitleResponse:
		return "EventType_SourceSubtitleResponse"
	case EventType_SourceSubtitleEnd:
		return "EventType_SourceSubtitleEnd"
	case EventType_TranslationSubtitleStart:
		return "EventType_TranslationSubtitleStart"
	case EventType_TranslationSubtitleResponse:
		return "EventType_TranslationSubtitleResponse"
	case EventType_TranslationSubtitleEnd:
		return "EventType_TranslationSubtitleEnd"
	default:
		return fmt.Sprintf("EventType_(%d)", t)
	}
}

// 0                 1                 2                 3
// | 0 1 2 3 4 5 6 7 | 0 1 2 3 4 5 6 7 | 0 1 2 3 4 5 6 7 | 0 1 2 3 4 5 6 7 |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |    Version      |   Header Size   |     Msg Type    |      Flags      |
// |   (4 bits)      |    (4 bits)     |     (4 bits)    |     (4 bits)    |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// | Serialization   |   Compression   |           Reserved                |
// |   (4 bits)      |    (4 bits)     |           (8 bits)                |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                                       |
// |                   Optional Header Extensions                          |
// |                     (if Header Size > 1)                              |
// |                                                                       |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
// |                                                                       |
// |                           Payload                                     |
// |                      (variable length)                                |
// |                                                                       |
// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

type Message struct {
	Version       VersionBits
	HeaderSize    HeaderSizeBits
	MsgType       MsgType
	MsgTypeFlag   MsgTypeFlagBits
	Serialization SerializationBits
	Compression   CompressionBits

	EventType EventType
	SessionID string
	ConnectID string
	Sequence  int32
	ErrorCode uint32

	Payload []byte
}

func NewMessageFromBytes(data []byte) (*Message, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("data too short: expected at least 3 bytes, got %d", len(data))
	}

	typeAndFlag := data[1]

	msg, err := NewMessage(MsgType(typeAndFlag>>4), MsgTypeFlagBits(typeAndFlag&0b00001111))
	if err != nil {
		return nil, err
	}

	if err := msg.Unmarshal(data); err != nil {
		return nil, err
	}

	return msg, nil
}

func NewMessage(msgType MsgType, flag MsgTypeFlagBits) (*Message, error) {
	return &Message{
		MsgType:       msgType,
		MsgTypeFlag:   flag,
		Version:       Version1,
		HeaderSize:    HeaderSize4,
		Serialization: SerializationJSON,
		Compression:   CompressionNone,
	}, nil
}

func (m *Message) String() string {
	switch m.MsgType {
	case MsgTypeAudioOnlyServer, MsgTypeAudioOnlyClient:
		if m.MsgTypeFlag == MsgTypeFlagPositiveSeq || m.MsgTypeFlag == MsgTypeFlagNegativeSeq {
			return fmt.Sprintf("%s, %s, Sequence: %d, PayloadSize: %d", m.MsgType, m.EventType, m.Sequence, len(m.Payload))
		}
		return fmt.Sprintf("%s, %s, PayloadSize: %d", m.MsgType, m.EventType, len(m.Payload))
	case MsgTypeError:
		return fmt.Sprintf("%s, %s, ErrorCode: %d, Payload: %s", m.MsgType, m.EventType, m.ErrorCode, string(m.Payload))
	default:
		if m.MsgTypeFlag == MsgTypeFlagPositiveSeq || m.MsgTypeFlag == MsgTypeFlagNegativeSeq {
			return fmt.Sprintf("%s, %s, Sequence: %d, Payload: %s",
				m.MsgType, m.EventType, m.Sequence, string(m.Payload))
		}
		return fmt.Sprintf("%s, %s, Payload: %s", m.MsgType, m.EventType, string(m.Payload))
	}
}

func (m *Message) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)

	header := []uint8{
		uint8(m.Version)<<4 | uint8(m.HeaderSize),
		uint8(m.MsgType)<<4 | uint8(m.MsgTypeFlag),
		uint8(m.Serialization)<<4 | uint8(m.Compression),
	}

	headerSize := 4 * int(m.HeaderSize)
	if padding := headerSize - len(header); padding > 0 {
		header = append(header, make([]uint8, padding)...)
	}

	if err := binary.Write(buf, binary.BigEndian, header); err != nil {
		return nil, err
	}

	writers, err := m.writers()
	if err != nil {
		return nil, err
	}

	for _, write := range writers {
		if err := write(buf); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func (m *Message) Unmarshal(data []byte) error {
	buf := bytes.NewBuffer(data)

	versionAndHeaderSize, err := buf.ReadByte()
	if err != nil {
		return err
	}

	m.Version = VersionBits(versionAndHeaderSize >> 4)
	m.HeaderSize = HeaderSizeBits(versionAndHeaderSize & 0b00001111)

	_, err = buf.ReadByte()
	if err != nil {
		return err
	}

	serializationCompression, err := buf.ReadByte()
	if err != nil {
		return err
	}

	m.Serialization = SerializationBits(serializationCompression & 0b11110000)
	m.Compression = CompressionBits(serializationCompression & 0b00001111)

	headerSize := 4 * int(m.HeaderSize)
	readSize := 3
	if paddingSize := headerSize - readSize; paddingSize > 0 {
		if n, err := buf.Read(make([]byte, paddingSize)); err != nil || n < paddingSize {
			return fmt.Errorf("insufficient header bytes: expected %d, got %d", paddingSize, n)
		}
	}

	readers, err := m.readers()
	if err != nil {
		return err
	}

	for _, read := range readers {
		if err := read(buf); err != nil {
			return err
		}
	}

	if _, err := buf.ReadByte(); err != io.EOF {
		return fmt.Errorf("unexpected data after message: %v", err)
	}

	return nil
}

func (m *Message) writers() (writers []func(*bytes.Buffer) error, _ error) {
	if m.MsgTypeFlag == MsgTypeFlagWithEvent {
		writers = append(writers, m.writeEvent, m.writeSessionID)
	}

	switch m.MsgType {
	case MsgTypeFullClientRequest, MsgTypeFullServerResponse, MsgTypeFrontEndResultServer, MsgTypeAudioOnlyClient, MsgTypeAudioOnlyServer:
		if m.MsgTypeFlag == MsgTypeFlagPositiveSeq || m.MsgTypeFlag == MsgTypeFlagNegativeSeq {
			writers = append(writers, m.writeSequence)
		}
	case MsgTypeError:
		writers = append(writers, m.writeErrorCode)
	default:
		return nil, fmt.Errorf("unsupported message type: %d", m.MsgType)
	}

	writers = append(writers, m.writePayload)
	return writers, nil
}

func (m *Message) writeEvent(buf *bytes.Buffer) error {
	return binary.Write(buf, binary.BigEndian, m.EventType)
}

func (m *Message) writeSessionID(buf *bytes.Buffer) error {
	switch m.EventType {
	case EventType_StartConnection, EventType_FinishConnection,
		EventType_ConnectionStarted, EventType_ConnectionFailed:
		return nil
	}

	size := len(m.SessionID)
	if size > math.MaxUint32 {
		return fmt.Errorf("session ID size (%d) exceeds max(uint32)", size)
	}

	if err := binary.Write(buf, binary.BigEndian, uint32(size)); err != nil {
		return err
	}

	buf.WriteString(m.SessionID)
	return nil
}

func (m *Message) writeSequence(buf *bytes.Buffer) error {
	return binary.Write(buf, binary.BigEndian, m.Sequence)
}

func (m *Message) writeErrorCode(buf *bytes.Buffer) error {
	return binary.Write(buf, binary.BigEndian, m.ErrorCode)
}

func (m *Message) writePayload(buf *bytes.Buffer) error {
	size := len(m.Payload)
	if size > math.MaxUint32 {
		return fmt.Errorf("payload size (%d) exceeds max(uint32)", size)
	}

	if err := binary.Write(buf, binary.BigEndian, uint32(size)); err != nil {
		return err
	}

	buf.Write(m.Payload)
	return nil
}

func (m *Message) readers() (readers []func(*bytes.Buffer) error, _ error) {
	switch m.MsgType {
	case MsgTypeFullClientRequest, MsgTypeFullServerResponse, MsgTypeFrontEndResultServer, MsgTypeAudioOnlyClient, MsgTypeAudioOnlyServer:
		if m.MsgTypeFlag == MsgTypeFlagPositiveSeq || m.MsgTypeFlag == MsgTypeFlagNegativeSeq {
			readers = append(readers, m.readSequence)
		}
	case MsgTypeError:
		readers = append(readers, m.readErrorCode)
	default:
		return nil, fmt.Errorf("unsupported message type: %d", m.MsgType)
	}

	if m.MsgTypeFlag == MsgTypeFlagWithEvent {
		readers = append(readers, m.readEvent, m.readSessionID, m.readConnectID)
	}

	readers = append(readers, m.readPayload)
	return readers, nil
}

func (m *Message) readEvent(buf *bytes.Buffer) error {
	return binary.Read(buf, binary.BigEndian, &m.EventType)
}

func (m *Message) readSessionID(buf *bytes.Buffer) error {
	switch m.EventType {
	case EventType_StartConnection, EventType_FinishConnection,
		EventType_ConnectionStarted, EventType_ConnectionFailed,
		EventType_ConnectionFinished:
		return nil
	}

	var size uint32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return err
	}

	if size > 0 {
		m.SessionID = string(buf.Next(int(size)))
	}

	return nil
}

func (m *Message) readConnectID(buf *bytes.Buffer) error {
	switch m.EventType {
	case EventType_ConnectionStarted, EventType_ConnectionFailed,
		EventType_ConnectionFinished:
	default:
		return nil
	}

	var size uint32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return err
	}

	if size > 0 {
		m.ConnectID = string(buf.Next(int(size)))
	}

	return nil
}

func (m *Message) readSequence(buf *bytes.Buffer) error {
	return binary.Read(buf, binary.BigEndian, &m.Sequence)
}

func (m *Message) readErrorCode(buf *bytes.Buffer) error {
	return binary.Read(buf, binary.BigEndian, &m.ErrorCode)
}

func (m *Message) readPayload(buf *bytes.Buffer) error {
	var size uint32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return err
	}

	if size > 0 {
		m.Payload = buf.Next(int(size))
	}

	return nil
}

func ReceiveMessage(conn *websocket.Conn) (*Message, error) {
	mt, frame, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if mt != websocket.BinaryMessage && mt != websocket.TextMessage {
		return nil, fmt.Errorf("unexpected Websocket message type: %d", mt)
	}
	msg, err := NewMessageFromBytes(frame)
	if err != nil {
		return nil, err
	}
	glog.Info("receive: ", msg)
	return msg, nil
}

func WaitForEvent(conn *websocket.Conn, msgType MsgType, eventType EventType) (*Message, error) {
	for {
		msg, err := ReceiveMessage(conn)
		if err != nil {
			return nil, err
		}
		if msg.MsgType != msgType || msg.EventType != eventType {
			return nil, fmt.Errorf("unexpected message: %s", msg)
		}
		if msg.MsgType == msgType && msg.EventType == eventType {
			return msg, nil
		}
	}
}

func FullClientRequest(conn *websocket.Conn, payload []byte) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagNoSeq)
	if err != nil {
		return err
	}
	msg.Payload = payload
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func AudioOnlyClient(conn *websocket.Conn, payload []byte, flag MsgTypeFlagBits) error {
	msg, err := NewMessage(MsgTypeAudioOnlyClient, flag)
	if err != nil {
		return err
	}
	msg.Payload = payload
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func StartConnection(conn *websocket.Conn) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_StartConnection
	msg.Payload = []byte("{}")
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func FinishConnection(conn *websocket.Conn) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_FinishConnection
	msg.Payload = []byte("{}")
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func StartSession(conn *websocket.Conn, payload []byte, sessionID string) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_StartSession
	msg.SessionID = sessionID
	msg.Payload = payload
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func FinishSession(conn *websocket.Conn, sessionID string) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_FinishSession
	msg.SessionID = sessionID
	msg.Payload = []byte("{}")
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func CancelSession(conn *websocket.Conn, sessionID string) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_CancelSession
	msg.SessionID = sessionID
	msg.Payload = []byte("{}")
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}

func TaskRequest(conn *websocket.Conn, payload []byte, sessionID string) error {
	msg, err := NewMessage(MsgTypeFullClientRequest, MsgTypeFlagWithEvent)
	if err != nil {
		return err
	}
	msg.EventType = EventType_TaskRequest
	msg.SessionID = sessionID
	msg.Payload = payload
	glog.Info("send: ", msg)
	frame, err := msg.Marshal()
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, frame)
}
