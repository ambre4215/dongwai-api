package handler

import (
	"net/http"
	"strings"
	"time"

	"dongwai_backend/internal/dto"
	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// --- DTO ---

type CreateCustomVocabReq struct {
	Name        string `json:"name" binding:"required"`
	Descript    string `json:"descript"`
	WordListStr string `json:"word_list_str"` // 逗号分隔的单词字符串
}

type UpdateSenseReq struct {
	VocabID string `json:"vocab_id" binding:"required"`
	SenseID string `json:"sense_id" binding:"required"` // 用户选中的 SenseID
}

// --- Handler ---

// CreateCustomVocabulary 创建自定义词书并导入单词
func CreateCustomVocabulary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateCustomVocabReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 1. 解析单词列表 (支持中文逗号、英文逗号、换行、空格)
		rawWords := strings.FieldsFunc(req.WordListStr, func(r rune) bool {
			return r == ',' || r == '，' || r == '\n' || r == ' '
		})

		// 去重
		uniqueWords := make(map[string]bool)
		var searchKeywords []string
		for _, w := range rawWords {
			w = strings.TrimSpace(w)
			if w != "" && !uniqueWords[w] {
				uniqueWords[w] = true
				searchKeywords = append(searchKeywords, w)
			}
		}

		if len(searchKeywords) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的单词列表"})
			return
		}

		// 2. 查找存在的单词
		var foundVocabs []model.Vocab
		if err := db.Select("id, kanji").Where("kanji IN ?", searchKeywords).Find(&foundVocabs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询词库失败"})
			return
		}

		if len(foundVocabs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "提供的单词在词库中均不存在，请先添加单词"})
			return
		}

		// 3. 准备数据
		vocabBookID := utils.GenerateID("vb_", req.Name, uuid.New().String())

		newBook := model.Vocabulary{
			ID:       vocabBookID,
			Name:     req.Name,
			Descript: req.Descript,
			Count:    len(foundVocabs),
			CreateAt: time.Now(),
			UpdataAt: time.Now(),
		}

		var relations []model.VocabularyWord
		for _, v := range foundVocabs {
			relations = append(relations, model.VocabularyWord{
				VocabularyID: vocabBookID,
				VocabID:      v.ID,
				SenseID:      "", // 初始为空，由用户后续选择
			})
		}

		// 4. 事务入库
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&newBook).Error; err != nil {
				return err
			}
			if len(relations) > 0 {
				if err := tx.CreateInBatches(&relations, 100).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      "词书创建成功",
			"id":           vocabBookID,
			"total_input":  len(searchKeywords),
			"valid_import": len(foundVocabs),
		})
	}
}

// GetVocabBookList 获取词书列表
func GetVocabBookList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var list []model.Vocabulary
		if err := db.Order("create_at DESC").Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
			return
		}

		// ✅ 使用统一 DTO 转换
		dtoList := make([]dto.VocabBookDTO, 0, len(list))
		for _, v := range list {
			dtoList = append(dtoList, dto.ToVocabBookDTO(v))
		}

		c.JSON(http.StatusOK, gin.H{"list": dtoList})
	}
}

// GetVocabBookDetail 获取词书详情 (核心：多义词优先排序)
func GetVocabBookDetail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		bookID := c.Param("id")

		// 简单分页
		page := 1
		pageSize := 100

		var relations []model.VocabularyWord

		// 关联查询逻辑:
		// 1. JOIN vocabs 表:为了获取 is_multi 字段进行排序
		// 2. Order is_multi DESC:多义词排在前面
		// 3. Preload Vocab.Senses:加载单词的所有释义,供前端展示和勾选
		err := db.
			Joins("JOIN vocabs ON vocabs.id = vocabulary_words.vocab_id").
			Where("vocabulary_words.vocabulary_id = ?", bookID).
			Preload("Vocab").
			Preload("Vocab.Senses").
			Preload("Vocab.Senses.Examples", func(db *gorm.DB) *gorm.DB {
				return db.Limit(2) // ✅ 每个 sense 最多 2 个例句
			}).
			Order("vocabs.is_multi DESC").             // 🔥 优先级1:多义词靠前
			Order("vocabulary_words.created_at DESC"). // 优先级2:后加入的靠前
			Offset((page - 1) * pageSize).
			Limit(pageSize).
			Find(&relations).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询详情失败"})
			return
		}

		// ✅ 使用统一 DTO 转换,简化返回结构
		words := make([]dto.VocabBookWordDTO, 0, len(relations))
		for _, rel := range relations {
			words = append(words, dto.ToVocabBookWordDTO(rel))
		}

		c.JSON(http.StatusOK, gin.H{
			"id":    bookID,
			"words": words, // ✅ 扁平化结构,前端更易使用
		})
	}
}

// UpdateBookWordSense 更新词书中单词选中的释义
func UpdateBookWordSense(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		bookID := c.Param("id")
		var req UpdateSenseReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 更新关联表中的 SenseID
		result := db.Model(&model.VocabularyWord{}).
			Where("vocabulary_id = ? AND vocab_id = ?", bookID, req.VocabID).
			Update("sense_id", req.SenseID)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "未找到该单词记录，可能不在当前词书中"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "已更新选中释义"})
	}
}
