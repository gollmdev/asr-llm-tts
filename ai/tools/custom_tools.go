package tools

import (
	"github.com/gollmdev/asr-llm-tts/ai/dag/node"
)

// CustomDefinitions defines business tools owned by the app (internal module).
func CustomDefinitions() []map[string]any {
	return []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "get_weather",
				"description": "当你想查询指定城市的天气时非常有用。",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]string{
							"type":        "string",
							"description": "城市或县区，比如北京市、杭州市、余杭区等。",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "get_user_report",
				"description": "查询当前登录用户自己的报告（用户ID由系统注入，禁止用户自行指定）。",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
	}
}

func CustomRegistry() map[string]node.ToolFunc {
	return map[string]node.ToolFunc{
		"get_weather":     GetWeatherTool,
		"get_user_report": GetUserReportTool,
	}
}

func RegisterCustom(executor *node.ToolExecutorNode, definitions []map[string]any) []string {
	if executor == nil {
		return nil
	}
	return executor.RegisterToolsFromDefinitions(definitions, CustomRegistry())
}
