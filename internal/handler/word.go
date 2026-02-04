package handler

import (
	"net/http"
	"time"

	"dongwai_backend/internal/dto"
	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/ai"
	"dongwai_backend/internal/pkg/cache"
	"dongwai_backend/internal/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// --- DTO ---

type WordExampleReq struct {
	Kanji    string `json:"kanji"`
	Def      string `json:"def"`
	Audio    string `json:"audio"`
	Furigana any    `json:"furigana"`
}

type WordSenseReq struct {
	ID       string           `json:"id"`
	Level    string           `json:"level"`
	Reading  string           `json:"reading"`
	Def      string           `json:"def"`
	Pos      string           `json:"pos"`
	Pitch    string           `json:"pitch"`
	Furigana any              `json:"furigana"`
	Examples []WordExampleReq `json:"examples"`
}

type CreateWordReq struct {
	Kanji   string         `json:"kanji" binding:"required"`
	IsMulti bool           `json:"is_multi"`
	Senses  []WordSenseReq `json:"senses"`
}

type UpdateWordReq struct {
	ID string `json:"id" binding:"required"`
	CreateWordReq
}

type ListWordsReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

type VocabSummary struct {
	ID      string `json:"id"`
	Kanji   string `json:"kanji"`
	Reading string `json:"reading"`
	Def     string `json:"def"`
	Level   string `json:"level"`
	IsMulti bool   `json:"is_multi"` // 可选：返回给前端以便展示不同图标
}

type ListWordsResp struct {
	Total int64          `json:"total"`
	List  []VocabSummary `json:"list"`
}

type WordDetailReq struct {
	ID string `json:"id" binding:"required"`
}

// --- Handler ---

// CreateWord 创建单词
func CreateWord(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateWordReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// AI 智能补全

		// 🔥 自动检测：如果义项数量大于1，则标记为多义词
		// 这覆盖了前端传来的值，也覆盖了 AI 的判断，确保数据库真实性
		req.IsMulti = len(req.Senses) > 1

		vocabID := utils.GenerateID("w_", req.Kanji, uuid.New().String())

		newVocab := model.Vocab{
			ID:       vocabID,
			Kanji:    req.Kanji,
			IsMulti:  req.IsMulti,
			CreatAt:  time.Now(),
			UpdataAt: time.Now(),
		}

		var senses []model.VocabSense
		var examples []model.SenseExample

		for _, s := range req.Senses {
			senseID := utils.GenerateID("s_", vocabID, uuid.New().String())
			newSense := model.VocabSense{
				ID:       senseID,
				VocabID:  vocabID,
				Level:    s.Level,
				Reading:  s.Reading,
				Def:      s.Def,
				Pos:      s.Pos,
				Pitch:    s.Pitch,
				Furigana: datatypes.JSON(utils.ToJSON(s.Furigana)),
			}
			senses = append(senses, newSense)
			for _, ex := range s.Examples {
				exID := utils.GenerateID("e_", senseID, uuid.New().String())
				newEx := model.SenseExample{
					ID:       exID,
					SenseID:  senseID,
					Kanji:    ex.Kanji,
					Def:      ex.Def,
					Audio:    ex.Audio,
					Furigana: datatypes.JSON(utils.ToJSON(ex.Furigana)),
				}
				examples = append(examples, newEx)
			}
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&newVocab).Error; err != nil {
				return err
			}
			if len(senses) > 0 {
				if err := tx.Create(&senses).Error; err != nil {
					return err
				}
			}
			if len(examples) > 0 {
				if err := tx.Create(&examples).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
			return
		}

		cache.GlobalDict.AddOrUpdate(req.Kanji, vocabID)

		c.JSON(http.StatusOK, gin.H{"id": vocabID, "message": "创建成功", "data": req})
	}
}

// UpdateWord 修改单词
func UpdateWord(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req UpdateWordReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 🔥 自动检测：更新时同样强制计算 IsMulti
		req.IsMulti = len(req.Senses) > 1

		var oldVocab model.Vocab
		if err := db.Select("kanji").First(&oldVocab, "id = ?", req.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "单词不存在"})
			return
		}
		oldKanji := oldVocab.Kanji

		err := db.Transaction(func(tx *gorm.DB) error {
			// 更新 Vocab 表 (包含自动计算的 IsMulti)
			if err := tx.Model(&model.Vocab{}).Where("id = ?", req.ID).Updates(map[string]interface{}{
				"kanji":     req.Kanji,
				"is_multi":  req.IsMulti,
				"updata_at": time.Now(),
			}).Error; err != nil {
				return err
			}

			// --- Sense 处理逻辑 ---
			var existingSenseIDs []string
			tx.Model(&model.VocabSense{}).Where("vocab_id = ?", req.ID).Pluck("id", &existingSenseIDs)
			existingMap := make(map[string]bool)
			for _, id := range existingSenseIDs {
				existingMap[id] = true
			}

			processedIDs := make(map[string]bool)

			for _, s := range req.Senses {
				var senseID string
				if s.ID != "" && existingMap[s.ID] {
					// 更新现有
					senseID = s.ID
					tx.Model(&model.VocabSense{}).Where("id = ?", senseID).Updates(model.VocabSense{
						Level:    s.Level,
						Reading:  s.Reading,
						Def:      s.Def,
						Pos:      s.Pos,
						Pitch:    s.Pitch,
						Furigana: datatypes.JSON(utils.ToJSON(s.Furigana)),
					})
					tx.Where("sense_id = ?", senseID).Delete(&model.SenseExample{})
				} else {
					// 新增
					senseID = utils.GenerateID("s_", req.ID, uuid.New().String())
					newSense := model.VocabSense{
						ID:       senseID,
						VocabID:  req.ID,
						Level:    s.Level,
						Reading:  s.Reading,
						Def:      s.Def,
						Pos:      s.Pos,
						Pitch:    s.Pitch,
						Furigana: datatypes.JSON(utils.ToJSON(s.Furigana)),
					}
					if err := tx.Create(&newSense).Error; err != nil {
						return err
					}
				}
				processedIDs[senseID] = true

				var newExamples []model.SenseExample
				for _, ex := range s.Examples {
					exID := utils.GenerateID("e_", senseID, uuid.New().String())
					newExamples = append(newExamples, model.SenseExample{
						ID:       exID,
						SenseID:  senseID,
						Kanji:    ex.Kanji,
						Def:      ex.Def,
						Audio:    ex.Audio,
						Furigana: datatypes.JSON(utils.ToJSON(ex.Furigana)),
					})
				}
				if len(newExamples) > 0 {
					if err := tx.Create(&newExamples).Error; err != nil {
						return err
					}
				}
			}

			// 删除未保留的 Sense
			for _, oldID := range existingSenseIDs {
				if !processedIDs[oldID] {
					tx.Where("sense_id = ?", oldID).Delete(&model.SenseExample{})
					tx.Where("id = ?", oldID).Delete(&model.VocabSense{})
				}
			}

			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
			return
		}

		if oldKanji != req.Kanji {
			cache.GlobalDict.Remove(oldKanji, req.ID)
		}
		cache.GlobalDict.AddOrUpdate(req.Kanji, req.ID)

		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

// DeleteWord 删除单词
func DeleteWord(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var vocab model.Vocab
		if err := db.First(&vocab, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "单词不存在"})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			var senseIDs []string
			tx.Model(&model.VocabSense{}).Where("vocab_id = ?", id).Pluck("id", &senseIDs)
			if len(senseIDs) > 0 {
				tx.Where("sense_id IN ?", senseIDs).Delete(&model.SenseExample{})
				tx.Where("id IN ?", senseIDs).Delete(&model.VocabSense{})
			}
			return tx.Delete(&model.Vocab{}, "id = ?", id).Error
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			return
		}

		cache.GlobalDict.Remove(vocab.Kanji, id)

		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}

// ListWords 分页获取列表
func ListWords(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ListWordsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Page = 1
			req.PageSize = 20
		}
		if req.Page < 1 {
			req.Page = 1
		}
		if req.PageSize < 1 {
			req.PageSize = 20
		}

		var total int64
		var vocabs []model.Vocab

		query := db.Model(&model.Vocab{})
		if req.Keyword != "" {
			query = query.Where("kanji LIKE ? OR id = ?", "%"+req.Keyword+"%", req.Keyword)
		}

		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}

		offset := (req.Page - 1) * req.PageSize

		// ✅ 性能优化:使用 Preload 批量加载 senses,避免 N+1 查询
		// ✅ 只加载前 3 个 senses,减少数据传输量
		err := query.
			Preload("Senses", func(db *gorm.DB) *gorm.DB {
				return db.Order("level ASC").Limit(3)
			}).
			Preload("Senses.Examples", func(db *gorm.DB) *gorm.DB {
				return db.Limit(2) // 每个 sense 最多 2 个例句
			}).
			Order("updata_at DESC").
			Offset(offset).
			Limit(req.PageSize).
			Find(&vocabs).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取列表失败"})
			return
		}

		// ✅ 使用统一 DTO 转换
		summaryList := make([]dto.WordSummaryDTO, 0, len(vocabs))
		for _, v := range vocabs {
			summaryList = append(summaryList, dto.ToWordSummaryDTO(v, 3))
		}

		c.JSON(http.StatusOK, gin.H{
			"total": total,
			"list":  summaryList,
		})
	}
}

// GetWordDetail 获取详情
func GetWordDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req WordDetailReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误，需要 id"})
			return
		}

		var vocab model.Vocab
		err := db.
			Preload("Senses").
			Preload("Senses.Examples").
			First(&vocab, "id = ?", req.ID).Error

		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "单词不存在"})
			return
		}

		// ✅ 使用统一 DTO 转换
		c.JSON(http.StatusOK, dto.ToWordDTO(vocab))
	}
}

// GenerateWordInfoHandler AI 自动生成单词信息 (不保存，仅返回给前端填充表单)
func GenerateWordInfoHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Kanji string `json:"kanji" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 Kanji"})
			return
		}

		// ✅ 传递上下文，支持取消
		aiData, err := ai.GenerateWordInfo(c.Request.Context(), req.Kanji)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI 生成失败: " + err.Error()})
			return
		}

		// 转换为前端友好的结构
		var senses []WordSenseReq
		for _, s := range aiData.Senses {
			var examples []WordExampleReq
			for _, ex := range s.Examples {
				examples = append(examples, WordExampleReq{
					Kanji:    ex.Kanji,
					Def:      ex.Def,
					Furigana: ex.Furigana,
				})
			}
			senses = append(senses, WordSenseReq{
				Level:    s.Level,
				Reading:  s.Reading,
				Def:      s.Def,
				Pos:      s.Pos,
				Pitch:    s.Pitch,
				Furigana: s.Furigana,
				Examples: examples,
			})
		}

		resp := CreateWordReq{
			Kanji:   req.Kanji,
			IsMulti: len(senses) > 1,
			Senses:  senses,
		}

		c.JSON(http.StatusOK, resp)
	}
}
