package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func GetUserReportTool(arguments string) (string, error) {
	payload := map[string]any{}
	trimmed := strings.TrimSpace(arguments)
	if trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return "", fmt.Errorf("无法解析用户报告参数: %w", err)
		}
	}

	userID, ok := pickSystemUserID(payload)
	if !ok || userID <= 0 {
		return "", fmt.Errorf("缺少系统注入的 userId，拒绝查询")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	report := map[string]any{
		"userId":      userID,
		"reportId":    fmt.Sprintf("GUT-%d", userID),
		"reportTitle": "肠道菌群健康检测报告（模拟）",
		"generatedAt": now,
		"summary": map[string]any{
			"overallScore":           82,
			"microbiomeDiversity":    "中等偏高",
			"beneficialBacteria":     "双歧杆菌占比偏低",
			"harmfulBacteriaRisk":    "低风险",
			"intestinalInflammation": "轻度风险",
			"metabolicStatus":        "良好",
			"sampleCollectedAt":      "2026-03-20",
		},
		"insights": []string{
			"建议增加高纤维食物摄入，如燕麦、豆类和深色蔬菜，以提升有益菌丰度。",
			"发酵食品摄入频率可提升至每周 3-5 次，帮助改善菌群稳定性。",
			"建议连续 4-6 周减少高糖高脂零食，观察炎症相关指标变化。",
		},
		"recommendations": []map[string]any{
			{"type": "diet", "content": "每日膳食纤维建议 25-30g，优先全谷物与蔬果。"},
			{"type": "lifestyle", "content": "保持规律作息，目标睡眠 7-8 小时/天。"},
			{"type": "follow_up", "content": "建议 8-12 周后复检肠道菌群。"},
		},
		"riskTags": []string{
			"菌群多样性轻度不足",
			"短链脂肪酸潜在不足",
		},
	}

	b, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("构建用户报告失败: %w", err)
	}
	return string(b), nil
}

func pickSystemUserID(payload map[string]any) (int64, bool) {
	raw, ok := payload["_systemUserId"]
	if !ok {
		return 0, false
	}
	id, ok := toInt64(raw)
	if !ok {
		return 0, false
	}
	return id, true
}

func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case string:
		if t == "" {
			return 0, false
		}
		id, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, false
		}
		return id, true
	default:
		return 0, false
	}
}
