package store

import (
	"regexp"
	"strings"
)

// summaryPatterns 自动摘要时需要清理的 markdown 语法。
var summaryPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile("(?s)```.*?```"), " "},          // 代码块
	{regexp.MustCompile("(?s)~~~.*?~~~"), " "},          // 代码块
	{regexp.MustCompile("`([^`]*)`"), "$1"},             // 行内代码
	{regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`), " "},   // 图片
	{regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`), "$1"}, // 链接保留文字
	{regexp.MustCompile(`(?m)^#{1,6}\s*`), ""},          // 标题符号
	{regexp.MustCompile(`(?m)^>\s?`), ""},               // 引用
	{regexp.MustCompile(`\|`), " "},                     // 表格分隔
	{regexp.MustCompile("[*_~]{1,3}"), ""},              // 强调
	{regexp.MustCompile(`<[^>]+>`), " "},                // HTML 标签
	{regexp.MustCompile(`\s+`), " "},                    // 空白折叠
}

// GenSummary 从 markdown 正文自动生成摘要（按字符截断，中文安全）。
func GenSummary(content string) string {
	text := strings.TrimSpace(content)
	for _, p := range summaryPatterns {
		text = p.re.ReplaceAllString(text, p.repl)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	const maxRunes = 110
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

// BackfillSummaries 为摘要为空的老文章批量生成摘要（启动时调用一次）。
func (s *Store) BackfillSummaries() error {
	rows, err := s.DB.Query(`SELECT id, content FROM posts WHERE summary = ''`)
	if err != nil {
		return err
	}
	type backfill struct {
		id      int64
		summary string
	}
	var items []backfill
	for rows.Next() {
		var it backfill
		var content string
		if err := rows.Scan(&it.id, &content); err != nil {
			rows.Close()
			return err
		}
		if sum := GenSummary(content); sum != "" {
			it.summary = sum
			items = append(items, it)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, it := range items {
		if _, err := s.DB.Exec(`UPDATE posts SET summary = ? WHERE id = ?`, it.summary, it.id); err != nil {
			return err
		}
	}
	return nil
}
