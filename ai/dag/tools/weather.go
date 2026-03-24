package tools

import (
	"encoding/json"
	"fmt"
)

func GetWeatherTool(arguments string) (string, error) {
	type weatherArgs struct {
		Location string `json:"location"`
	}

	var args weatherArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("无法解析天气查询参数: %w", err)
	}
	if args.Location == "" {
		return "", fmt.Errorf("缺少 location 参数")
	}

	return fmt.Sprintf("%s当前天气晴，气温15度。", args.Location), nil
}
