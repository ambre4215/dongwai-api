package model

import (
	"time"

	"gorm.io/datatypes"
)

type Vocab struct {
	ID       string `gorm:"primary;type:varchar(32)"`
	Kanji    string `gorm:"index;not null"`
	IsMulti  bool   `gorm:"type:bool"`
	CreatAt  time.Time
	UpdataAt time.Time
	Senses   []VocabSense `gorm:"foreignKey:VocabID"`
}

type VocabSense struct {
	ID       string         `gorm:"primary;type:varchar(32)"`
	Level    string         `gorm:"index;type:varchar(5)"`
	VocabID  string         `gorm:"index"`
	Reading  string         `gorm:"index;not null"`
	Furigana datatypes.JSON `gorm:"type:jsonb"`
	Pitch    string         `gorm:"type:varchar(10)"`
	Pos      string         `gorm:"type:varchar(50)"`
	Def      string         `gorm:"text"`
	Audio    string         `gorm:"varchar(255)"`
	Examples []SenseExample `gorm:"foreignKey:SenseID"`
}

type SenseExample struct {
	ID       string         `gorm:"primary;type:varchar(32)"`
	SenseID  string         `gorm:"index"`
	Kanji    string         `gorm:"type:text"`
	Furigana datatypes.JSON `gorm:"type:jsonb"`
	Def      string         `gorm:"type:text"`
	Audio    string         `gorm:"type:varchar(255)"`
}
