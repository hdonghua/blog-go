package web

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"blog-go/internal/store"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.Table, extension.Linkify, extension.TaskList, extension.Strikethrough),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

// pageItem 分页条目，Ellipsis 为 true 时显示省略号。
type pageItem struct {
	Num      int
	URL      string
	Current  bool
	Ellipsis bool
}

// pageSize 前台分页条数：优先读 settings 表，回落到配置文件。
func (s *Server) pageSize() int {
	size := s.cfg.Site.PageSize
	if m, err := s.store.GetSettings(); err == nil {
		if v, err := strconv.Atoi(strings.TrimSpace(m["page_size"])); err == nil && v > 0 && v <= 100 {
			size = v
		}
	}
	if size <= 0 {
		size = 20
	}
	return size
}

func pageURL(q string, cat int64, page int) string {
	v := url.Values{}
	if q != "" {
		v.Set("q", q)
	}
	if cat > 0 {
		v.Set("cat", strconv.FormatInt(cat, 10))
	}
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if enc := v.Encode(); enc != "" {
		return "/?" + enc
	}
	return "/"
}

// buildPageNums 构建页码条目，页数多时使用 1 … p-1 p p+1 … N 窗口。
func buildPageNums(q string, cat int64, page, totalPages int) []pageItem {
	if totalPages <= 1 {
		return nil
	}
	var items []pageItem
	add := func(n int) {
		items = append(items, pageItem{Num: n, URL: pageURL(q, cat, n), Current: n == page})
	}
	if totalPages <= 9 {
		for i := 1; i <= totalPages; i++ {
			add(i)
		}
		return items
	}
	add(1)
	start := page - 2
	if start < 2 {
		start = 2
	}
	end := page + 2
	if end > totalPages-1 {
		end = totalPages - 1
	}
	if start > 2 {
		items = append(items, pageItem{Ellipsis: true})
	}
	for i := start; i <= end; i++ {
		add(i)
	}
	if end < totalPages-1 {
		items = append(items, pageItem{Ellipsis: true})
	}
	add(totalPages)
	return items
}

func (s *Server) frontIndex(c *gin.Context) {
	q := c.Query("q")
	cat, _ := strconv.ParseInt(c.Query("cat"), 10, 64)
	title, icp, favicon := s.siteInfo()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	size := s.pageSize()
	total, err := s.store.CountPosts(q, cat)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	totalPages := (total + size - 1) / size
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	posts, err := s.store.ListPosts(q, cat, size, (page-1)*size)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	cats, _ := s.store.CategoriesWithCount()
	for i := range cats {
		cats[i].Active = cats[i].ID == cat
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"SiteTitle":  title,
		"ICP":        icp,
		"Favicon":    favicon,
		"Query":      q,
		"Posts":      posts,
		"Total":      total,
		"Cats":       cats,
		"AllActive":  cat == 0,
		"CurCat":     cat,
		"Page":       page,
		"TotalPages": totalPages,
		"PrevURL":    pageURL(q, cat, page-1),
		"NextURL":    pageURL(q, cat, page+1),
		"PageNums":   buildPageNums(q, cat, page, totalPages),
	})
}

func (s *Server) frontPost(c *gin.Context) {
	title, icp, favicon := s.siteInfo()
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	post, err := s.store.GetPost(id)
	if err == store.ErrNotFound {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"SiteTitle": title, "Favicon": favicon})
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	var buf strings.Builder
	if err := md.Convert([]byte(post.Content), &buf); err != nil {
		c.String(http.StatusInternalServerError, "渲染文章失败: %v", err)
		return
	}
	c.HTML(http.StatusOK, "post.html", gin.H{
		"SiteTitle":   title,
		"ICP":         icp,
		"Favicon":     favicon,
		"Post":        post,
		"ContentHTML": template.HTML(buf.String()),
	})
}
