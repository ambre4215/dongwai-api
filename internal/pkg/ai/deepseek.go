package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dongwai_backend/internal/config"
)

// --- 通用结构体 ---

type deepseekReq struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
	JsonMode bool      `json:"json_mode,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekResp struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// --- 功能一：文章单词消歧 ---

// Candidate 表示一个待消歧的单词及其选项
type Candidate struct {
	WordID   string
	WordText string
	Context  string   // 单词所在的句子
	Options  []string // 候选释义列表
}

// BatchDisambiguate 批量消歧
func BatchDisambiguate(ctx context.Context, candidates []Candidate) map[string]int {
	if config.AppConfig.DEEPSEEK_API_KEY == "" || len(candidates) == 0 {
		return nil
	}

	resultMap := make(map[string]int)

	var promptBuilder strings.Builder
	// 🔥 优化点：增强提示词，教 AI 识别接头/接尾辞标记
	promptBuilder.WriteString("你是一位日语词典专家。请根据提供的句子上下文，从选项中识别单词的正确释义。\n")
	promptBuilder.WriteString("特别注意：\n")
	promptBuilder.WriteString("1. 选项中可能包含 [原词] 标记（例如 [～的] 表示接尾辞，[御～] 表示接头辞）。\n")
	promptBuilder.WriteString("2. 请务必分析上下文的语法结构（如前接名词、后接动词等），判断该词是作为独立词、接头辞还是接尾辞使用。\n")
	promptBuilder.WriteString("3. 请仅返回一个 JSON 对象，其中键是 WordID，值是最合适释义的索引（从0开始的整数）。\n\n")

	for _, c := range candidates {
		promptBuilder.WriteString(fmt.Sprintf("WordID: %s\n单词: %s\n上下文: %s\n选项:\n", c.WordID, c.WordText, c.Context))
		for i, opt := range c.Options {
			promptBuilder.WriteString(fmt.Sprintf("%d. %s\n", i, opt))
		}
		promptBuilder.WriteString("---\n")
	}

	reqBody := deepseekReq{
		Model: "deepseek-chat",
		Messages: []message{
			{Role: "system", Content: "你是一个只输出 JSON 的日语助手。"}, // 简化 system prompt，主要指令在 user prompt
			{Role: "user", Content: promptBuilder.String()},
		},
		Stream:   false,
		JsonMode: true,
	}

	jsonData, _ := json.Marshal(reqBody)
	// ✅ 使用 NewRequestWithContext 支持取消
	req, _ := http.NewRequestWithContext(ctx, "POST", config.AppConfig.DEEPSEEK_BASE_URL+"/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.DEEPSEEK_API_KEY)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("DeepSeek API error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var apiResp deepseekResp
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("DeepSeek decode error: %v", err)
		return nil
	}

	if len(apiResp.Choices) > 0 {
		content := apiResp.Choices[0].Message.Content
		content = cleanJSON(content)
		if err := json.Unmarshal([]byte(content), &resultMap); err != nil {
			log.Printf("DeepSeek JSON parse error: %v | Content: %s", err, content)
		}
	}

	return resultMap
}

// --- 功能二：单词智能补全 (含 Furigana) ---

// GeneratedWordData AI 生成的单词结构
type GeneratedWordData struct {
	Kanji   string `json:"kanji"`
	IsMulti bool   `json:"is_multi"`
	Senses  []struct {
		Level    string     `json:"level"`    // N1-N5
		Reading  string     `json:"reading"`  // 平假名
		Furigana [][]string `json:"furigana"` // ✅ 单词本身的振假名拆解
		Pitch    string     `json:"pitch"`    // 音调，如 ⓪ ①
		Pos      string     `json:"pos"`      // 词性
		Def      string     `json:"def"`      // 释义
		Examples []struct {
			Kanji    string     `json:"kanji"`    // 例句原文
			Furigana [][]string `json:"furigana"` // ✅ 例句的振假名拆解
			Def      string     `json:"def"`      // 例句翻译
		} `json:"examples"`
	} `json:"senses"`
}

// GenerateWordInfo 调用 AI 自动补全单词信息
func GenerateWordInfo(ctx context.Context, word string) (*GeneratedWordData, error) {
	if config.AppConfig.DEEPSEEK_API_KEY == "" {
		return nil, fmt.Errorf("DeepSeek API Key 未配置")
	}

	// 构造中文 Prompt
	// 重点：要求 furigana 必须为 [[text, reading]] 格式
	prompt := fmt.Sprintf(`
你是一位专业的日语词典编辑。请为日语单词 "%s" 生成详细的词典条目。

要求：
1. 严格按照下方的 JSON 格式输出。
2. "pitch"（音调）: 必须使用带圈数字表示音调核（例如：⓪, ①, ②）。
3. "examples"（例句）: 每个释义最多 2 个例句。
4. "level": JLPT 等级 (N1-N5)，必须根据单词难度准确评估，不可为 null。
5. "reading": 单词的平假名读音。
6. "def"（释义）: 使用**中文**简洁准确地解释。
7. "pos"（词性）: 使用常见的**中文**词性名称。
8.151→8. 🔥 "furigana"（振假名）: **必须**输出为二维数组格式 [[文本, 读音], [文本, 读音]]。
152→   - 汉字部分必须标注读音。
153→   - 假名部分读音留空字符串 ""。
154→   - 即使单词本身全是假名，也要拆分为二维数组格式，例如 "こんにちは" -> [["こんにちは", ""]]。
155→   - 例如 "猫が好き" -> [["猫", "ねこ"], ["が", ""], ["好き", "すき"]]。

JSON 结构示例：
{
  "kanji": "%s",
  "is_multi": false,
  "senses": [
    {
      "level": "N5",
      "reading": "ねこ",
      "furigana": [["猫", "ねこ"]],
      "pitch": "⓪",
      "pos": "名词",
      "def": "猫，一种宠物。",
      "examples": [
        { 
           "kanji": "猫が好きです", 
           "furigana": [["猫", "ねこ"], ["が", ""], ["好き", "すき"], ["です", ""]],
           "def": "我喜欢猫。" 
        }
      ]
    }
  ]
}
`, word, word)

	reqBody := deepseekReq{
		Model: "deepseek-chat",
		Messages: []message{
			{Role: "system", Content: "你是一个乐于助人的助手，请严格只输出 JSON 格式。"},
			{Role: "user", Content: prompt},
		},
		Stream:   false,
		JsonMode: true,
	}

	jsonData, _ := json.Marshal(reqBody)
	// ✅ 使用 NewRequestWithContext 支持取消
	req, _ := http.NewRequestWithContext(ctx, "POST", config.AppConfig.DEEPSEEK_BASE_URL+"/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AppConfig.DEEPSEEK_API_KEY)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp deepseekResp
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("AI return empty choices")
	}

	content := apiResp.Choices[0].Message.Content
	content = cleanJSON(content)

	var result GeneratedWordData
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("AI JSON Parse Error: %v \nContent: %s", err, content)
		return nil, err
	}

	return &result, nil
}

func cleanJSON(content string) string {
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}
