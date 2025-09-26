package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"protocols"
	"strings"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	flagAppid       = flag.String("appid", "", "appid")
	flagAccessToken = flag.String("access_token", "", "access_token")
	flagResourceId  = flag.String("resource_id", "", "resource_id")
	flagVoiceType   = flag.String("voice_type", "", "voice_type")
	flagText        = flag.String("text", "", "text")
	flagEncoding    = flag.String("encoding", "wav", "encoding")
	flagEndpoint    = flag.String("endpoint", "wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream", "endpoint")
)

func VoiceToResourceId(voice string) string {
	if strings.HasPrefix(voice, "S_") {
		return "volc.megatts.default"
	}
	return "volc.service_type.10029"
}

func main() {
	flag.Set("v", "3")
	flag.Set("logtostderr", "true")
	flag.Parse()

	voice := *flagVoiceType
	header := http.Header{}
	header.Set("X-Api-App-Key", *flagAppid)
	header.Set("X-Api-Access-Key", *flagAccessToken)
	if *flagResourceId != "" {
		header.Set("X-Api-Resource-Id", *flagResourceId)
	} else {
		header.Set("X-Api-Resource-Id", VoiceToResourceId(voice))
	}
	header.Set("X-Api-Connect-Id", uuid.New().String())
	// ----------------dial server----------------
	conn, r, err := websocket.DefaultDialer.DialContext(context.Background(), *flagEndpoint, header)
	if err != nil {
		glog.Exit(r, err)
	}
	defer conn.Close()
	glog.Info("Connection established, Logid:", r.Header.Get("x-tt-logid"))

	encoding := *flagEncoding
	request := map[string]any{
		"user": map[string]any{
			"uid": uuid.New().String(),
		},
		"req_params": map[string]any{
			"speaker": voice,
			"audio_params": map[string]any{
				"format":           encoding,
				"sample_rate":      24000,
				"enable_timestamp": true,
			},
			"text": *flagText,
			"additions": func() string {
				str, _ := json.Marshal(map[string]any{
					"disable_markdown_filter": false,
				})
				return string(str)
			}(),
		},
	}
	payload, err := json.Marshal(&request)
	if err != nil {
		glog.Exit(err)
	}
	// ----------------send text----------------
	if err := protocols.FullClientRequest(conn, payload); err != nil {
		glog.Exit(err)
	}
	// ----------------wait for result----------------
	var audio []byte
	for {
		msg, err := protocols.ReceiveMessage(conn)
		if err != nil {
			glog.Exit(err)
		}
		switch msg.MsgType {
		case protocols.MsgTypeFullServerResponse:
		case protocols.MsgTypeAudioOnlyServer:
			audio = append(audio, msg.Payload...)
		default:
			glog.Exit(msg)
		}
		if msg.MsgType == protocols.MsgTypeFullServerResponse && msg.EventType == protocols.EventType_SessionFinished {
			break
		}
	}
	if len(audio) == 0 {
		glog.Exit("audio is empty")
	}
	fileName := voice + "." + string(encoding)
	if err := os.WriteFile(fileName, audio, 0644); err != nil {
		glog.Exit(err)
	}
	glog.Infof("audio received: %d, saved to %s", len(audio), fileName)
}
