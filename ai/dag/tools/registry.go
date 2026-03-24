package tools

import "github.com/gollmdev/asr-llm-tts/ai/dag/node"

func BuiltinRegistry() map[string]node.ToolFunc {
	return map[string]node.ToolFunc{
		"get_weather": GetWeatherTool,
	}
}

func RegisterBuiltins(executor *node.ToolExecutorNode, definitions []map[string]any) []string {
	if executor == nil {
		return nil
	}
	return executor.RegisterToolsFromDefinitions(definitions, BuiltinRegistry())
}
