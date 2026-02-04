package main

import (
	"log"

	"dongwai_backend/internal/config"
	"dongwai_backend/internal/handler"
	"dongwai_backend/internal/model"
	"dongwai_backend/internal/pkg/auth"
	"dongwai_backend/internal/pkg/cache"
	"dongwai_backend/internal/pkg/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	config.LoadConfig()
	auth.InitJWT(config.AppConfig.JWT_SECRET)

	// 连接数据库
	db, err := gorm.Open(postgres.Open(config.AppConfig.DB_DSN), &gorm.Config{})
	if err != nil {
		log.Fatal("无法连接数据库: ", err)
	}

	// 自动迁移
	// ✅ 确保包含了 model.Vocabulary 和 model.VocabularyWord
	err = db.AutoMigrate(
		&model.UserRole{},
		&model.Vocab{},
		&model.VocabSense{},
		&model.SenseExample{},
		&model.Vocabulary{},     // 词书表
		&model.VocabularyWord{}, // 词书-单词关联表
	)
	if err != nil {
		log.Fatal("表结构迁移失败: ", err)
	}

	// 初始化词典缓存
	log.Println("正在加载词典缓存...")
	if err := cache.InitDictCache(db); err != nil {
		log.Fatal("缓存初始化失败: ", err)
	}
	log.Println("✅ 词典缓存加载完毕")

	// 配置路由
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH") // 增加了 PATCH
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.POST("/login", handler.Login(db))

	api := r.Group("/api")
	{
		api.POST("/analyze", handler.AnalyzeArticle(db))

		authorized := api.Group("/")
		authorized.Use(middleware.JWTAuth())
		{
			// === 单词管理 ===
			authorized.POST("/word/generate", handler.GenerateWordInfoHandler()) // ✅ 新增 AI 生成接口
			authorized.POST("/word", handler.CreateWord(db))
			authorized.PUT("/word", handler.UpdateWord(db))
			authorized.DELETE("/word/:id", handler.DeleteWord(db))
			authorized.POST("/word/list", handler.ListWords(db))
			authorized.POST("/word/detail", handler.GetWordDetail(db))

			// === ✅ 词书管理 ===
			// 创建自定义词书 (导入逗号分隔的字符串)
			authorized.POST("/vocab-book", handler.CreateCustomVocabulary(db))

			// 获取所有词书列表
			authorized.GET("/vocab-book", handler.GetVocabBookList(db))

			// 获取词书详情 (优先显示多义词)
			authorized.GET("/vocab-book/:id", handler.GetVocabBookDetail(db))

			// 更新词书中某个单词选中的释义 (勾选操作)
			authorized.PUT("/vocab-book/:id/word", handler.UpdateBookWordSense(db))
		}
	}

	log.Printf("服务器启动在 http://localhost:%s", config.AppConfig.PORT)
	r.Run(":" + config.AppConfig.PORT)
}
