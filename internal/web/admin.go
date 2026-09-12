package web

import (
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog-go/internal/auth"
	"blog-go/internal/store"
)

func (s *Server) loginPage(c *gin.Context) {
	title, _, favicon := s.siteInfo()
	// 已登录用户访问登录页时直接进入后台
	if _, ok := s.currentUser(c); ok {
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	c.HTML(http.StatusOK, "login.html", gin.H{
		"SiteTitle": title,
		"Favicon":   favicon,
		"Error":     c.Query("err"),
	})
}

func (s *Server) loginSubmit(c *gin.Context) {
	title, _, favicon := s.siteInfo()
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	fail := func(msg string) {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"SiteTitle": title,
			"Favicon":   favicon,
			"Error":     msg,
			"Username":  username,
		})
	}
	u, err := s.store.GetUserByUsername(username)
	if err != nil || !auth.CheckPassword(u.PasswordHash, password) {
		fail("用户名或密码错误")
		return
	}
	ttl := time.Duration(s.cfg.Auth.SessionHours) * time.Hour
	token := auth.MakeToken(u.ID, ttl, s.cfg.Auth.Secret)
	c.SetCookie(s.cfg.Auth.CookieName, token, int(ttl.Seconds()), "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin")
}

func (s *Server) logout(c *gin.Context) {
	c.SetCookie(s.cfg.Auth.CookieName, "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/admin/login")
}

// requireLogin cookie 验证：校验 HMAC 签名与有效期。
func (s *Server) requireLogin(c *gin.Context) {
	if _, ok := s.currentUser(c); !ok {
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
		return
	}
	c.Next()
}

// currentUser 从 cookie 解析当前登录用户 ID。
func (s *Server) currentUser(c *gin.Context) (int64, bool) {
	token, err := c.Cookie(s.cfg.Auth.CookieName)
	if err != nil || token == "" {
		return 0, false
	}
	return auth.ParseToken(token, s.cfg.Auth.Secret)
}

func (s *Server) adminHome(c *gin.Context) {
	title, _, favicon := s.siteInfo()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	const adminPageSize = 10
	total, err := s.store.CountAdminPosts()
	if err != nil {
		c.String(http.StatusInternalServerError, "统计文章失败: %v", err)
		return
	}
	totalPages := (total + adminPageSize - 1) / adminPageSize
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	posts, err := s.store.ListAdminPosts(adminPageSize, (page-1)*adminPageSize)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	cats, _ := s.store.Categories()
	adminURL := func(n int) string { return "/admin?page=" + strconv.Itoa(n) }
	c.HTML(http.StatusOK, "admin_home.html", gin.H{
		"SiteTitle":  title,
		"Favicon":    favicon,
		"Posts":      posts,
		"Total":      total,
		"Cats":       cats,
		"Page":       page,
		"TotalPages": totalPages,
		"PrevURL":    adminURL(page - 1),
		"NextURL":    adminURL(page + 1),
		"PageNums":   buildPageNums(page, totalPages, adminURL),
	})
}

func (s *Server) postNewPage(c *gin.Context) {
	title, _, favicon := s.siteInfo()
	cats, err := s.store.Categories()
	if err != nil {
		c.String(http.StatusInternalServerError, "读取分类失败: %v", err)
		return
	}
	c.HTML(http.StatusOK, "post_new.html", gin.H{
		"SiteTitle": title,
		"Favicon":   favicon,
		"Cats":      cats,
		"Error":     "",
		"Title":     "",
		"Content":   "",
		"Summary":   "",
		"CategoryID": int64(0),
		"PublishedAt": "",
		"Status": store.StatusPublished,
		"EditID": int64(0),
	})
}

// postForm 发布/编辑文章的表单数据。
type postForm struct {
	Title         string
	Content       string
	Summary       string
	CategoryID    int64
	PublishedAt   time.Time
	PublishedAtStr string
	Status        int
}

// parsePostForm 解析发布/编辑表单。
func (s *Server) parsePostForm(c *gin.Context) (postForm, error) {
	f := postForm{Status: store.StatusPublished}
	f.Title = strings.TrimSpace(c.PostForm("title"))
	f.Content = c.PostForm("content") // editor.md 的 markdown 原文
	f.Summary = strings.TrimSpace(c.PostForm("summary"))
	f.CategoryID, _ = strconv.ParseInt(c.PostForm("category_id"), 10, 64)
	// 选择“未分类”时自动归入“其它”分类，避免外键约束错误
	if f.CategoryID == 0 {
		var err error
		f.CategoryID, err = s.store.GetOrCreateCategoryByName("其它")
		if err != nil {
			return f, err
		}
	}
	// 发布日期：后台可选，未填时默认当前日期
	f.PublishedAt = time.Now()
	f.PublishedAtStr = strings.TrimSpace(c.PostForm("published_at"))
	if f.PublishedAtStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", f.PublishedAtStr, time.Local); err == nil {
			f.PublishedAt = t
		}
	}
	if c.PostForm("status") == "0" {
		f.Status = store.StatusDraft
	}
	return f, nil
}


// renderPostForm 渲染发布/编辑表单页（校验失败时回显）。
func (s *Server) renderPostForm(c *gin.Context, errMsg string, editID int64, f postForm) {
	title, _, favicon := s.siteInfo()
	cats, _ := s.store.Categories()
	c.HTML(http.StatusBadRequest, "post_new.html", gin.H{
		"SiteTitle": title,
		"Favicon":   favicon,
		"Cats":      cats,
		"Error":     errMsg,
		"EditID":    editID,
		"Title":     f.Title,
		"Content":   f.Content,
		"Summary":   f.Summary,
		"CategoryID": f.CategoryID,
		"PublishedAt": f.PublishedAtStr,
		"Status":    f.Status,
	})
}

func (s *Server) postCreate(c *gin.Context) {
	f, err := s.parsePostForm(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "创建默认分类失败: %v", err)
		return
	}
	if f.Title == "" || f.Content == "" {
		s.renderPostForm(c, "标题和内容不能为空", 0, f)
		return
	}
	if f.Summary == "" {
		f.Summary = store.GenSummary(f.Content) // 摘要留空时从正文自动生成
	}
	id, err := s.store.CreatePost(f.Title, f.CategoryID, f.Content, f.Summary, f.PublishedAt, f.Status)
	if err != nil {
		c.String(http.StatusInternalServerError, "保存文章失败: %v", err)
		return
	}
	if f.Status == store.StatusDraft {
		c.Redirect(http.StatusFound, "/admin")
	} else {
		c.Redirect(http.StatusFound, "/post/"+strconv.FormatInt(id, 10))
	}
}

// postEditPage 编辑文章页（含草稿）。
func (s *Server) postEditPage(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	title, _, favicon := s.siteInfo()
	post, err := s.store.GetPostForEdit(id)
	if err == store.ErrNotFound {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"SiteTitle": title, "Favicon": favicon})
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	cats, err := s.store.Categories()
	if err != nil {
		c.String(http.StatusInternalServerError, "读取分类失败: %v", err)
		return
	}
	c.HTML(http.StatusOK, "post_new.html", gin.H{
		"SiteTitle": title,
		"Favicon":   favicon,
		"Cats":      cats,
		"Error":     "",
		"EditID":    id,
		"Title":     post.Title,
		"Content":   post.Content,
		"Summary":   post.Summary,
		"CategoryID": post.CategoryID,
		"PublishedAt": post.CreatedAt.Format("2006-01-02"),
		"Status":    post.Status,
	})
}

// postUpdate 保存编辑后的文章。
func (s *Server) postUpdate(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	pageTitle, _, favicon := s.siteInfo()
	f, err := s.parsePostForm(c)
	if err != nil {
		c.String(http.StatusInternalServerError, "创建默认分类失败: %v", err)
		return
	}
	if f.Title == "" || f.Content == "" {
		s.renderPostForm(c, "标题和内容不能为空", id, f)
		return
	}
	if f.Summary == "" {
		f.Summary = store.GenSummary(f.Content)
	}
	err = s.store.UpdatePost(id, f.Title, f.CategoryID, f.Content, f.Summary, f.PublishedAt, f.Status)
	if err == store.ErrNotFound {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"SiteTitle": pageTitle, "Favicon": favicon})
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "保存文章失败: %v", err)
		return
	}
	if f.Status == store.StatusDraft {
		c.Redirect(http.StatusFound, "/admin")
	} else {
		c.Redirect(http.StatusFound, "/post/"+strconv.FormatInt(id, 10))
	}
}

// postExport 导出文章 markdown 原文，文件名为「文章标题.md」。
func (s *Server) postExport(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	post, err := s.store.GetPostForEdit(id)
	if err == store.ErrNotFound {
		c.HTML(http.StatusNotFound, "404.html", gin.H{"SiteTitle": "404"})
		return
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "读取文章失败: %v", err)
		return
	}
	// 清理文件名中的非法字符（Windows: \ / : * ? " < > | 及控制符）
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 32 {
			return -1
		}
		return r
	}, strings.TrimSpace(post.Title))
	if safe == "" {
		safe = "文章-" + strconv.FormatInt(id, 10)
	}
	filename := safe + ".md"
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.String(http.StatusOK, post.Content)
}

func (s *Server) categoriesPage(c *gin.Context) {
	title, _, favicon := s.siteInfo()
	cats, err := s.store.Categories()
	if err != nil {
		c.String(http.StatusInternalServerError, "读取分类失败: %v", err)
		return
	}
	c.HTML(http.StatusOK, "categories.html", gin.H{
		"SiteTitle": title,
		"Favicon":   favicon,
		"Cats":      cats,
		"Error":     c.Query("err"),
		"Saved":     c.Query("saved"),
	})
}

func (s *Server) categoryCreate(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.Redirect(http.StatusFound, "/admin/categories?err=empty")
		return
	}
	if len([]rune(name)) > 30 {
		c.Redirect(http.StatusFound, "/admin/categories?err=long")
		return
	}
	sortOrder, ok := parseSortOrder(c.PostForm("sort_order"))
	if !ok {
		c.Redirect(http.StatusFound, "/admin/categories?err=sort")
		return
	}
	if _, err := s.store.CreateCategory(name, sortOrder); err != nil {
		c.Redirect(http.StatusFound, "/admin/categories?err=duplicate")
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories")
}

// categoryUpdate 批量保存分类名称与排序号。
func (s *Server) categoryUpdate(c *gin.Context) {
	cats, err := s.store.Categories()
	if err != nil {
		c.String(http.StatusInternalServerError, "读取分类失败: %v", err)
		return
	}
	items := make([]store.Category, 0, len(cats))
	seen := map[string]struct{}{}
	for _, cat := range cats {
		idStr := strconv.FormatInt(cat.ID, 10)
		name := strings.TrimSpace(c.PostForm("name_" + idStr))
		if name == "" {
			c.Redirect(http.StatusFound, "/admin/categories?err=empty")
			return
		}
		if len([]rune(name)) > 30 {
			c.Redirect(http.StatusFound, "/admin/categories?err=long")
			return
		}
		if _, ok := seen[name]; ok {
			c.Redirect(http.StatusFound, "/admin/categories?err=duplicate")
			return
		}
		seen[name] = struct{}{}
		sortOrder, ok := parseSortOrder(c.PostForm("sort_" + idStr))
		if !ok {
			c.Redirect(http.StatusFound, "/admin/categories?err=sort")
			return
		}
		items = append(items, store.Category{ID: cat.ID, Name: name, SortOrder: sortOrder})
	}
	if err := s.store.UpdateCategories(items); err == store.ErrExists {
		c.Redirect(http.StatusFound, "/admin/categories?err=duplicate")
		return
	} else if err != nil {
		c.String(http.StatusInternalServerError, "保存分类失败: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/categories?saved=1")
}

// parseSortOrder 解析排序号，空值视为 0。
func parseSortOrder(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// settingsPage 站点设置：网站标题、备案号。
func (s *Server) settingsPage(c *gin.Context) {
	title, icp, favicon := s.siteInfo()
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"SiteTitle":    title,
		"Favicon":      favicon,
		"SettingTitle": title,
		"SettingICP":   icp,
		"SettingFavicon": favicon,
		"SettingPageSize": s.pageSize(),
		"Error":        c.Query("err"),
		"Saved":        c.Query("saved"),
	})
}

func (s *Server) settingsSave(c *gin.Context) {
	title := strings.TrimSpace(c.PostForm("site_title"))
	icp := strings.TrimSpace(c.PostForm("icp"))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("page_size")))
	render := func(errMsg string) {
		pageTitle, _, favicon := s.siteInfo()
		c.HTML(http.StatusBadRequest, "settings.html", gin.H{
			"SiteTitle":    pageTitle,
			"Favicon":      favicon,
			"SettingTitle": title,
			"SettingICP":   icp,
			"SettingPageSize": pageSize,
			"Error":        errMsg,
		})
	}
	if title == "" {
		render("网站标题不能为空")
		return
	}
	if err := s.store.SetSetting("site_title", title); err != nil {
		c.String(http.StatusInternalServerError, "保存设置失败: %v", err)
		return
	}
	if err := s.store.SetSetting("icp", icp); err != nil {
		c.String(http.StatusInternalServerError, "保存设置失败: %v", err)
		return
	}
	if pageSize < 1 || pageSize > 100 {
		render("分页条数需在 1-100 之间")
		return
	}
	if err := s.store.SetSetting("page_size", strconv.Itoa(pageSize)); err != nil {
		c.String(http.StatusInternalServerError, "保存设置失败: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings?saved=1")
}

// settingsFaviconUpload 上传网站图标（浏览器标签页图标）。
func (s *Server) settingsFaviconUpload(c *gin.Context) {
	imgURL, err := s.saveUploadedImage(c, "favicon")
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape(err.Error()))
		return
	}
	if err := s.store.SetSetting("favicon", imgURL); err != nil {
		c.String(http.StatusInternalServerError, "保存网站图标失败: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings?saved=1")
}

const minPasswordLen = 6

// settingsPassword 修改当前登录用户密码。
func (s *Server) settingsPassword(c *gin.Context) {
	uid, ok := s.currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/login")
		return
	}
	current := c.PostForm("current_password")
	next := c.PostForm("new_password")
	confirm := c.PostForm("confirm_password")
	render := func(errMsg string) {
		title, icp, favicon := s.siteInfo()
		c.HTML(http.StatusBadRequest, "settings.html", gin.H{
			"SiteTitle":       title,
			"Favicon":         favicon,
			"SettingTitle":    title,
			"SettingICP":      icp,
			"SettingFavicon":  favicon,
			"SettingPageSize": s.pageSize(),
			"Error":           errMsg,
		})
	}
	if current == "" || next == "" || confirm == "" {
		render("请填写当前密码和新密码")
		return
	}
	if next != confirm {
		render("两次输入的新密码不一致")
		return
	}
	if len(next) < minPasswordLen {
		render("新密码至少 6 位")
		return
	}
	if next == current {
		render("新密码不能与当前密码相同")
		return
	}
	u, err := s.store.GetUserByID(uid)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取用户失败: %v", err)
		return
	}
	if !auth.CheckPassword(u.PasswordHash, current) {
		render("当前密码不正确")
		return
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		c.String(http.StatusInternalServerError, "加密密码失败: %v", err)
		return
	}
	if err := s.store.UpdateUserPassword(uid, hash); err != nil {
		c.String(http.StatusInternalServerError, "保存密码失败: %v", err)
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings?saved=pwd")
}
