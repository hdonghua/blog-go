package web

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"blog-go/internal/config"
	"blog-go/internal/store"
)

type Server struct {
	cfg   *config.Config
	store *store.Store
}

func New(cfg *config.Config, st *store.Store) *Server {
	return &Server{cfg: cfg, store: st}
}

// siteInfo 从 settings 表读取网站标题、备案号和图标，标题为空时回落到配置文件。
func (s *Server) siteInfo() (title, icp, favicon string) {
	title = s.cfg.Site.Title
	if m, err := s.store.GetSettings(); err == nil {
		if v := strings.TrimSpace(m["site_title"]); v != "" {
			title = v
		}
		icp = strings.TrimSpace(m["icp"])
		favicon = strings.TrimSpace(m["favicon"])
	}
	return title, icp, favicon
}

// seoInfo 读取 SEO 设置（meta description / keywords）。
func (s *Server) seoInfo() (description, keywords string) {
	if m, err := s.store.GetSettings(); err == nil {
		description = strings.TrimSpace(m["seo_description"])
		keywords = strings.TrimSpace(m["seo_keywords"])
	}
	return
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.SetFuncMap(template.FuncMap{
		"fmttime": func(t time.Time) string { return t.Format("2006-01-02") },
	})
	r.LoadHTMLGlob("templates/*/*.html")
	r.Static("/static", "static")
	r.Static("/uploads", s.cfg.Upload.Dir)

	// 前台
	r.GET("/", s.frontIndex)
	r.GET("/post/:id", s.frontPost)
	r.GET("/sitemap.xml", s.sitemapXML)
	r.GET("/robots.txt", s.robotsTXT)

	// 图片 API（上传需登录，展示公开）
	r.POST("/api/upload", s.requireLogin, s.uploadImage)

	// 后台
	r.GET("/admin/login", s.loginPage)
	r.POST("/admin/login", s.loginSubmit)
	r.POST("/admin/logout", s.logout)

	admin := r.Group("/admin", s.requireLogin)
	{
		admin.GET("", s.adminHome)
		admin.GET("/posts/new", s.postNewPage)
		admin.POST("/posts", s.postCreate)
		admin.GET("/edit/:id", s.postEditPage)
		admin.POST("/edit/:id", s.postUpdate)
		admin.GET("/export/:id", s.postExport)
		admin.GET("/categories", s.categoriesPage)
		admin.POST("/categories", s.categoryCreate)
		admin.POST("/categories/update", s.categoryUpdate)
		admin.GET("/settings", s.settingsPage)
		admin.POST("/settings", s.settingsSave)
		admin.POST("/settings/seo", s.settingsSeoSave)
		admin.POST("/settings/favicon", s.settingsFaviconUpload)
		admin.POST("/settings/password", s.settingsPassword)
	}

	r.NoRoute(func(c *gin.Context) {
		title, _, favicon := s.siteInfo()
		c.HTML(http.StatusNotFound, "404.html", gin.H{"SiteTitle": title, "Favicon": favicon})
	})
	return r
}
