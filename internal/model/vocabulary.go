package model

import (
	"time"
)

// Vocabulary 词书表
type Vocabulary struct {
	ID       string `gorm:"primaryKey;type:varchar(32)"`
	Name     string `gorm:"not null"`
	Descript string `gorm:"text"`
	// 记录词书中单词的总数
	Count    int `gorm:"default:0"`
	CreateAt time.Time
	UpdataAt time.Time
}

// VocabularyWord 词书与单词的关联表 (多对多)
type VocabularyWord struct {
	VocabularyID string `gorm:"primaryKey;type:varchar(32);index"`
	VocabID      string `gorm:"primaryKey;type:varchar(32);index"`

	// ✅ 核心字段：记录用户在词书中勾选的特定 SenseID
	// 如果为空字符串，表示用户尚未指定具体释义
	SenseID string `gorm:"type:varchar(32);index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`

	// 关联 Vocab，方便 Preload 查询
	Vocab Vocab `gorm:"foreignKey:VocabID"`
}
