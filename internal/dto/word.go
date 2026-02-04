package dto

import (
	"dongwai_backend/internal/model"
)

// ========================================
// 单词相关 DTO
// ========================================

// WordDTO 完整单词信息(用于详情页、词书详情等)
type WordDTO struct {
	ID      string     `json:"id"`
	Kanji   string     `json:"kanji"`
	IsMulti bool       `json:"is_multi"`
	Senses  []SenseDTO `json:"senses"`
}

// WordSummaryDTO 单词摘要信息(用于列表页,性能优化)
type WordSummaryDTO struct {
	ID      string     `json:"id"`
	Kanji   string     `json:"kanji"`
	IsMulti bool       `json:"is_multi"`
	Senses  []SenseDTO `json:"senses"` // 只包含前 N 个 senses
}

// SenseDTO 义项信息
type SenseDTO struct {
	ID       string       `json:"id"`
	Level    string       `json:"level"`
	Reading  string       `json:"reading"`
	Pos      string       `json:"pos"`
	Def      string       `json:"def"`
	Pitch    string       `json:"pitch"`
	Furigana interface{}  `json:"furigana"`
	Examples []ExampleDTO `json:"examples"`
}

// ExampleDTO 例句信息
type ExampleDTO struct {
	Kanji    string      `json:"kanji"`
	Def      string      `json:"def"`
	Furigana interface{} `json:"furigana"`
	Audio    string      `json:"audio,omitempty"`
}

// ========================================
// 转换函数
// ========================================

// ToWordDTO 将 model.Vocab 转换为完整的 WordDTO
func ToWordDTO(vocab model.Vocab) WordDTO {
	return WordDTO{
		ID:      vocab.ID,
		Kanji:   vocab.Kanji,
		IsMulti: vocab.IsMulti,
		Senses:  toSenseDTOs(vocab.Senses),
	}
}

// ToWordSummaryDTO 将 model.Vocab 转换为摘要 WordSummaryDTO
// maxSenses: 最多返回的 senses 数量(性能优化)
func ToWordSummaryDTO(vocab model.Vocab, maxSenses int) WordSummaryDTO {
	senses := vocab.Senses
	if len(senses) > maxSenses {
		senses = senses[:maxSenses]
	}

	return WordSummaryDTO{
		ID:      vocab.ID,
		Kanji:   vocab.Kanji,
		IsMulti: vocab.IsMulti,
		Senses:  toSenseDTOs(senses),
	}
}

// toSenseDTOs 将 []model.VocabSense 转换为 []SenseDTO
func toSenseDTOs(senses []model.VocabSense) []SenseDTO {
	result := make([]SenseDTO, 0, len(senses))

	for _, sense := range senses {
		senseDTO := SenseDTO{
			ID:       sense.ID,
			Level:    sense.Level,
			Reading:  sense.Reading,
			Pos:      sense.Pos,
			Def:      sense.Def,
			Pitch:    sense.Pitch,
			Furigana: sense.Furigana,
			Examples: toExampleDTOs(sense.Examples),
		}
		result = append(result, senseDTO)
	}

	return result
}

// toExampleDTOs 将 []model.SenseExample 转换为 []ExampleDTO
func toExampleDTOs(examples []model.SenseExample) []ExampleDTO {
	result := make([]ExampleDTO, 0, len(examples))

	for _, ex := range examples {
		exampleDTO := ExampleDTO{
			Kanji:    ex.Kanji,
			Def:      ex.Def,
			Furigana: ex.Furigana,
			Audio:    ex.Audio,
		}
		result = append(result, exampleDTO)
	}

	return result
}

// ========================================
// 词书相关 DTO
// ========================================

// VocabBookDTO 词书基本信息
type VocabBookDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Descript string `json:"descript"`
	Count    int    `json:"count"`
}

// VocabBookWordDTO 词书中的单词信息
type VocabBookWordDTO struct {
	VocabID         string  `json:"vocab_id"`
	SelectedSenseID string  `json:"selected_sense_id"` // 用户选中的义项 ID
	Word            WordDTO `json:"word"`
}

// ToVocabBookDTO 将 model.Vocabulary 转换为 VocabBookDTO
func ToVocabBookDTO(vocab model.Vocabulary) VocabBookDTO {
	return VocabBookDTO{
		ID:       vocab.ID,
		Name:     vocab.Name,
		Descript: vocab.Descript,
		Count:    vocab.Count,
	}
}

// ToVocabBookWordDTO 将 model.VocabularyWord 转换为 VocabBookWordDTO
func ToVocabBookWordDTO(relation model.VocabularyWord) VocabBookWordDTO {
	return VocabBookWordDTO{
		VocabID:         relation.VocabID,
		SelectedSenseID: relation.SenseID,
		Word:            ToWordDTO(relation.Vocab),
	}
}
