package model

import "time"

type UserRole struct {
	ID       string `gorm:"primary;type:varchar(36)"`
	Username string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"not null"`

	// ✅ 新增：角色字段 (存储 "admin", "super_admin", "editor" 等)
	Role string `gorm:"type:varchar(20);not null;default:'admin'"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
