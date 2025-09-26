package speech

import (
	"fmt"
	"strings"

	speechmodel "github.com/zhouzirui/z-tavern/backend/internal/model/speech"
)

// resolveCredentials 返回规范化后的 AppID 与 AccessToken，缺失时给出明确错误。
func resolveCredentials(cfg *speechmodel.SpeechConfig) (string, string, error) {
	if cfg == nil {
		return "", "", fmt.Errorf("火山引擎语音配置未初始化")
	}

	appID := strings.TrimSpace(cfg.AppID)
	token := strings.TrimSpace(cfg.AccessToken)
	if token == "" {
		token = strings.TrimSpace(cfg.APIKey)
	}

	if appID == "" || token == "" {
		return "", "", fmt.Errorf("火山引擎语音配置缺少 AppID 或 AccessToken")
	}

	return appID, token, nil
}
