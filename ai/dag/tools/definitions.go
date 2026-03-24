package tools

// BuiltinDefinitions returns tool schemas exposed to the LLM.
func BuiltinDefinitions() []map[string]any {
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
	}
}
