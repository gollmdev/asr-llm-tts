package test

import (
	"testing"

	engine "github.com/gollmdev/asr-llm-tts/ai/dag"
	dag "github.com/gollmdev/asr-llm-tts/ai/dag/core"
)

func TestDAG(t *testing.T) {
	dag.Test()
}

func TestDAG2(t *testing.T) {
	engine.Test2()
}
