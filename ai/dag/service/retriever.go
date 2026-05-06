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
			ID:       "bifido_001",
			Content:  "双歧杆菌（Bifidobacterium）是一种革兰氏阳性、厌氧、呈Y形或棒状的非运动型细菌。它是人体肠道（尤其是母乳喂养的婴儿肠道）中的核心共生菌群，属于放线菌门。",
			Score:    0.98,
			Source:   "microbiology_taxonomy_db",
			Title:    "双歧杆菌的分类学特征",
			Metadata: map[string]any{"author": "Microbiology Dept", "type": "definition"},
		},
		{
			ID:       "bifido_002",
			Content:  "双歧杆菌通过独特的‘果糖-6-磷酸旁路’（F6P pathway）代谢碳水化合物。该途径能将葡萄糖转化为乙酸和乳酸，且乙酸与乳酸的摩尔比为3:2，这是区分双歧杆菌与其他乳酸菌的关键生化特征。",
			Score:    0.96,
			Source:   "biochemistry_journal",
			Title:    "双歧杆菌的代谢机制",
			Metadata: map[string]any{"author": "Dr. S. Kim", "year": 2022},
		},
		{
			ID:       "bifido_003",
			Content:  "作为重要的益生菌，双歧杆菌能产生抗菌物质（如细菌素），竞争性排斥致病菌（如沙门氏菌、大肠杆菌），并通过增强肠道上皮屏障功能来降低肠道通透性，从而预防‘肠漏症’。",
			Score:    0.94,
			Source:   "probiotic_research",
			Title:    "双歧杆菌的益生功能与屏障保护",
			Metadata: map[string]any{"author": "Gut Health Lab", "tags": []string{"probiotics", "gut barrier"}},
		},
		{
			ID:       "bifido_004",
			Content:  "双歧杆菌的丰度随年龄增长而下降。百岁老人体内的双歧杆菌数量显著少于年轻人，这种减少与慢性炎症水平升高（炎性衰老）及免疫功能下降密切相关。补充双歧杆菌可能有助于延缓衰老相关的免疫衰退。",
			Score:    0.91,
			Source:   "aging_study_2023",
			Title:    "双歧杆菌与人类衰老的关系",
			Metadata: map[string]any{"author": "Longevity Institute", "focus": "aging"},
		},
		{
			ID:       "bifido_005",
			Content:  "人乳低聚糖（HMOs）是母乳中复杂的糖类，婴儿无法直接消化。双歧杆菌（特别是婴儿双歧杆菌）拥有特定的基因簇，能分解HMOs作为唯一碳源，这解释了为何母乳喂养儿肠道中双歧杆菌占据绝对优势。",
			Score:    0.89,
			Source:   "pediatric_nutrition",
			Title:    "双歧杆菌与母乳喂养的协同进化",
			Metadata: map[string]any{"author": "Dr. Mills", "topic": "infant nutrition"},
		},
	}
	if topK < len(docs) {
		return docs[:topK], nil
	}
	return docs, nil
}
