package handler

import (
	"fmt"
	"net/http"
	"strings"

	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/ai"
	"dongwai_backend/internal/pkg/cache" // 引入缓存包

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ... DTO 结构体保持不变 (ExampleDetail, WordDetail, Token, AnalyzeResp, WordResult) ...
// (为了节省篇幅，这里省略 DTO 定义，请保留原文件中的 struct 定义)

type ExampleDetail struct {
	Kanji    string `json:"kanji"`
	Def      string `json:"def"`
	Audio    string `json:"audio"`
	Furigana any    `json:"furigana"`
}

type WordDetail struct {
	VocabID  string          `json:"vocab_id"` // ✅ 新增
	SenseID  string          `json:"sense_id"`
	Level    string          `json:"level"`
	Reading  string          `json:"reading"`
	Def      string          `json:"def"`
	Pos      string          `json:"pos"`
	Selected bool            `json:"selected"`
	Examples []ExampleDetail `json:"examples"`
}

type Token struct {
	Text       string       `json:"text"`
	IsWord     bool         `json:"is_word"`
	Detail     *WordDetail  `json:"detail"`
	Candidates []WordDetail `json:"candidates"`
}

type AnalyzeResp struct {
	Tokens    []Token      `json:"tokens"`
	VocabList []WordResult `json:"vocab_list"`
}

type WordResult struct {
	Text       string       `json:"text"`
	Detail     WordDetail   `json:"detail"`
	Candidates []WordDetail `json:"candidates"`
}

type senseOptionRef struct {
	VocabID string
	Sense   model.VocabSense
}

// AnalyzeArticle 分析文章接口
func AnalyzeArticle(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供文章内容"})
			return
		}

		// ==========================================
		// 1. 使用内存缓存 (性能优化 ✅)
		// ==========================================
		// 不再查库，直接从 GlobalDict 获取
		maxLen := cache.GlobalDict.MaxLen()

		// ==========================================
		// 2. FMM (正向最大匹配) 分词
		// ==========================================
		runes := []rune(req.Content)
		length := len(runes)
		var tokens []Token

		tokenVocabIDsMap := make(map[int][]string)
		allFoundIDs := make(map[string]bool)

		for i := 0; i < length; {
			matched := false
			limit := i + maxLen
			if limit > length {
				limit = length
			}

			for j := limit; j > i; j-- {
				word := string(runes[i:j])
				// ⚡️ 从缓存查询
				if ids, exists := cache.GlobalDict.Get(word); exists {
					tokenVocabIDsMap[len(tokens)] = ids
					for _, id := range ids {
						allFoundIDs[id] = true
					}

					tokens = append(tokens, Token{Text: word, IsWord: true})
					i = j
					matched = true
					break
				}
			}

			if !matched {
				tokens = append(tokens, Token{Text: string(runes[i : i+1]), IsWord: false})
				i++
			}
		}

		// ==========================================
		// 3. 批量查询详情 (查库只查命中部分)
		// ==========================================
		if len(allFoundIDs) == 0 {
			// ✅ 没有找到单词,但仍然返回 SSE 格式
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

			emptyResp := AnalyzeResp{Tokens: tokens, VocabList: []WordResult{}}
			c.SSEvent("initial", emptyResp)
			c.Writer.Flush()
			return
		}

		var ids []string
		for id := range allFoundIDs {
			ids = append(ids, id)
		}

		var vocabsFull []model.Vocab
		db.Preload("Senses").Preload("Senses.Examples").Where("id IN ?", ids).Find(&vocabsFull)

		vocabObjMap := make(map[string]model.Vocab)
		for _, v := range vocabsFull {
			vocabObjMap[v.ID] = v
		}

		// ==========================================
		// 4. 准备 AI 消歧候选集 (逻辑不变)
		// ==========================================
		var aiCandidates []ai.Candidate

		for idx, t := range tokens {
			if !t.IsWord {
				continue
			}
			candidateIDs := tokenVocabIDsMap[idx]
			if len(candidateIDs) == 0 {
				continue
			}

			var currentOptions []senseOptionRef
			var optionsText []string

			for _, vid := range candidateIDs {
				v, ok := vocabObjMap[vid]
				if !ok {
					continue
				}
				rawWordLabel := fmt.Sprintf("[%s]", v.Kanji)

				for _, s := range v.Senses {
					currentOptions = append(currentOptions, senseOptionRef{VocabID: vid, Sense: s})
					defShort := s.Def
					if len(defShort) > 50 {
						defShort = defShort[:50] + "..."
					}
					optStr := fmt.Sprintf("%s [%s] %s - %s", rawWordLabel, s.Level, s.Pos, defShort)
					optionsText = append(optionsText, optStr)
				}
			}

			if len(currentOptions) > 1 {
				uniqueKey := fmt.Sprintf("token_%d", idx)
				contextStr := getContext(tokens, idx, 15)
				aiCandidates = append(aiCandidates, ai.Candidate{
					WordID:   uniqueKey,
					WordText: t.Text,
					Context:  contextStr,
					Options:  optionsText,
				})
			}
		}

		// ==========================================
		// 5. 设置 SSE Header 并发送初始结果 (纯逻辑)
		// ==========================================
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		// ❌ 移除重复的 Access-Control-Allow-Origin (中间件已设置)
		// c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// 构建初始响应（不带 AI 结果，默认选第一个）
		emptyAIResult := make(map[string]int)
		initialResp := buildAnalyzeResp(tokens, tokenVocabIDsMap, vocabObjMap, emptyAIResult)

		c.SSEvent("initial", initialResp)
		c.Writer.Flush()

		// ==========================================
		// 6. 后台执行 AI 消歧并推送更新
		// ==========================================
		if len(aiCandidates) > 0 {
			// ✅ 检查客户端是否断开连接
			select {
			case <-c.Request.Context().Done():
				// 客户端已断开，不再执行耗时的 AI 操作
				return
			default:
				// 继续执行
			}

			// ✅ 传递上下文，支持取消
			aiResult := ai.BatchDisambiguate(c.Request.Context(), aiCandidates)

			// 再次检查连接状态，防止写入 closed pipe
			select {
			case <-c.Request.Context().Done():
				return
			default:
				c.SSEvent("ai_update", aiResult)
				c.Writer.Flush()
			}
		}
	}
}

// buildAnalyzeResp 构建响应数据 (提取为独立函数以便复用逻辑)
func buildAnalyzeResp(tokens []Token, tokenVocabIDsMap map[int][]string, vocabObjMap map[string]model.Vocab, aiResult map[string]int) AnalyzeResp {
	var resultVocabList []WordResult
	vocabListSet := make(map[string]bool)

	// 深拷贝 tokens 以免修改原始 slice 影响后续逻辑（虽然这里是一次性的，但是个好习惯）
	// 但在这里我们直接修改传入的 tokens 副本（因为是 slice，修改元素会影响底层，但我们每次都重新构建 Detail）
	// 为了安全，我们创建一个新的 slice
	finalTokens := make([]Token, len(tokens))
	copy(finalTokens, tokens)

	for idx, t := range finalTokens {
		if !t.IsWord {
			continue
		}
		candidateIDs := tokenVocabIDsMap[idx]
		if len(candidateIDs) == 0 {
			continue
		}

		// 重建选项 (必须与准备 AI 候选集时的顺序一致)
		var allOptions []senseOptionRef
		for _, vid := range candidateIDs {
			if v, ok := vocabObjMap[vid]; ok {
				for _, s := range v.Senses {
					allOptions = append(allOptions, senseOptionRef{VocabID: vid, Sense: s})
				}
			}
		}

		if len(allOptions) == 0 {
			continue
		}

		selectedIndex := 0
		uniqueKey := fmt.Sprintf("token_%d", idx)
		if aiIdx, ok := aiResult[uniqueKey]; ok {
			if aiIdx >= 0 && aiIdx < len(allOptions) {
				selectedIndex = aiIdx
			}
		}

		var candidates []WordDetail
		var bestDetail *WordDetail

		for i, opt := range allOptions {
			isSelected := (i == selectedIndex)
			var examples []ExampleDetail
			for _, ex := range opt.Sense.Examples {
				examples = append(examples, ExampleDetail{
					Kanji:    ex.Kanji,
					Def:      ex.Def,
					Audio:    ex.Audio,
					Furigana: ex.Furigana,
				})
			}

			detail := WordDetail{
				VocabID:  opt.VocabID, // ✅ 赋值
				SenseID:  opt.Sense.ID,
				Level:    opt.Sense.Level,
				Reading:  opt.Sense.Reading,
				Def:      opt.Sense.Def,
				Pos:      opt.Sense.Pos,
				Selected: isSelected,
				Examples: examples,
			}
			candidates = append(candidates, detail)
			if isSelected {
				d := detail
				bestDetail = &d
			}
		}

		if bestDetail == nil && len(candidates) > 0 {
			d := candidates[0]
			bestDetail = &d
		}

		finalTokens[idx].Detail = bestDetail
		finalTokens[idx].Candidates = candidates

		// 侧边栏
		if bestDetail != nil {
			selectedVocabText := vocabObjMap[allOptions[selectedIndex].VocabID].Kanji
			if !vocabListSet[selectedVocabText] {
				resultVocabList = append(resultVocabList, WordResult{
					Text:       selectedVocabText,
					Detail:     *bestDetail,
					Candidates: candidates,
				})
				vocabListSet[selectedVocabText] = true
			}
		}
	}

	return AnalyzeResp{
		Tokens:    finalTokens,
		VocabList: resultVocabList,
	}
}

// getContext 保持不变
func getContext(tokens []Token, currentIdx int, rangeVal int) string {
	start := currentIdx - rangeVal
	if start < 0 {
		start = 0
	}
	end := currentIdx + rangeVal
	if end > len(tokens) {
		end = len(tokens)
	}
	var sb strings.Builder
	for i := start; i < end; i++ {
		sb.WriteString(tokens[i].Text)
	}
	return sb.String()
}
