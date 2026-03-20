package service

import "github.com/gollmdev/asr-llm-tts/ai/dag/dagtypes"

type Retriever interface {
	Search(query string, topK int) ([]dagtypes.RetrievedDoc, error)
}

type MockRetriever struct{}

func (r *MockRetriever) Search(query string, topK int) ([]dagtypes.RetrievedDoc, error) {
	// 返回一些mock数据
	docs := []dagtypes.RetrievedDoc{
		{
			ID:       "doc1",
			Content:  "这是第一条检索到的文档，内容与查询相关。",
			Score:    0.9,
			Source:   "mock_source",
			Title:    "文档1",
			Metadata: map[string]any{"author": "Alice"},
		},
		{
			ID:       "doc2",
			Content:  "这是第二条检索到的文档，也与查询相关。",
			Score:    0.85,
			Source:   "mock_source",
			Title:    "文档2",
			Metadata: map[string]any{"author": "Bob"},
		},
	}
	if topK < len(docs) {
		return docs[:topK], nil
	}
	return docs, nil
}
