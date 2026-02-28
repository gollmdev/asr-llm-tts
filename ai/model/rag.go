package model

type ThoughtItem struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type ThoughtChain struct {
	Status string        `json:"status"`
	Title  string        `json:"title"`
	Items  []ThoughtItem `json:"items"`
}

// []struct {
// 		Title   string `json:"title"`
// 		Content string `json:"content"`
// 	} `json:"items"`
// Citations
// {
//     "12345679": {
//         "title": "低剂量阿司匹林对PI3K突变结直肠癌的影响研究综述",
//         "number": 1,
//         "chunk_id": "12345679"
//     },
//     "89454131": {
//         "title": "SAKK 41/13试验：辅助阿司匹林在PIK3CA突变结肠癌中的疗效分析",
//         "number": 2,
//         "chunk_id": "89454131"
//     }
// }

type Citations struct {
	Title   string `json:"title"`
	Number  int    `json:"number"`
	ChunkID string `json:"chunk_id"`
}
