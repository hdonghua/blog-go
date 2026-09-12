package web

import (
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// sitemapURLSet / sitemapURL sitemap.xml 结构。
type sitemapURLSet struct {
	XMLName xml.Name      `xml:"urlset"`
	XMLNs   string        `xml:"xmlns,attr"`
	URLs    []sitemapURL  `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod,omitempty"`
}

// baseURL 从请求推导站点根地址（支持反代 X-Forwarded-Proto）。
func baseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if v := c.GetHeader("X-Forwarded-Proto"); v != "" {
		scheme = v
	}
	return scheme + "://" + c.Request.Host
}

// sitemapXML 自动生成站点地图：首页 + 全部已发布文章。
func (s *Server) sitemapXML(c *gin.Context) {
	base := baseURL(c)
	posts, err := s.store.ListPostSitemap()
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 sitemap 失败: %v", err)
		return
	}
	set := sitemapURLSet{
		XMLNs: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  []sitemapURL{{Loc: base + "/"}},
	}
	for _, p := range posts {
		set.URLs = append(set.URLs, sitemapURL{
			Loc:     base + "/post/" + strconv.FormatInt(p.ID, 10),
			Lastmod: p.UpdatedAt.UTC().Format("2006-01-02"),
		})
	}
	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "生成 sitemap 失败: %v", err)
		return
	}
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, xml.Header+string(out))
}

// robotsTXT 禁止抓取后台与 API，并声明 sitemap 地址。
func (s *Server) robotsTXT(c *gin.Context) {
	body := "User-agent: *\n" +
		"Disallow: /admin\n" +
		"Disallow: /api\n" +
		"\n" +
		"Sitemap: " + baseURL(c) + "/sitemap.xml\n"
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}
