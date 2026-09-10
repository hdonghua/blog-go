package store

import "time"

// 文章状态
const (
	StatusDraft     = 0 // 草稿，前台不显示
	StatusPublished = 1 // 正式发布
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Category struct {
	ID        int64
	Name      string
	SortOrder int
}

// CategoryCount 分类及文章数量（前台分类导航用）。
type CategoryCount struct {
	ID        int64
	Name      string
	SortOrder int
	Count     int
	Active    bool
}

type Post struct {
	ID           int64
	Title        string
	CategoryID   int64
	CategoryName string
	Content      string // markdown 原文
	Status       int    // 0=草稿 1=正式发布
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
