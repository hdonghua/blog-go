package web

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const faviconSize = 64

// buildFaviconICO 将上传图片等比缩放到 64x64 居中，编码 PNG 并包装为 ICO 容器。
func buildFaviconICO(src []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("无法识别的图片格式: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("无效图片尺寸")
	}
	// 等比缩放到 faviconSize 内
	sw, sh := faviconSize, faviconSize
	if w > h {
		sh = h * faviconSize / w
	} else {
		sw = w * faviconSize / h
	}
	if sw < 1 {
		sw = 1
	}
	if sh < 1 {
		sh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, faviconSize, faviconSize))
	offX, offY := (faviconSize-sw)/2, (faviconSize-sh)/2
	draw.CatmullRom.Scale(dst, image.Rect(offX, offY, offX+sw, offY+sh), img, b, draw.Over, nil)

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, dst); err != nil {
		return nil, fmt.Errorf("编码 PNG 失败: %w", err)
	}

	// ICO 容器：ICONDIR(6字节) + ICONDIRENTRY(16字节) + PNG 数据
	var ico bytes.Buffer
	ico.Write([]byte{0, 0, 1, 0, 1, 0}) // reserved=0, type=1(icon), count=1
	ico.Write([]byte{byte(faviconSize), byte(faviconSize), 0, 0, 1, 0, 32, 0})
	_ = binary.Write(&ico, binary.LittleEndian, uint32(pngBuf.Len()))
	_ = binary.Write(&ico, binary.LittleEndian, uint32(22))
	ico.Write(pngBuf.Bytes())
	return ico.Bytes(), nil
}

// settingsFaviconUpload 上传网站图标：自动转成 64x64 的 favicon.ico 固定文件名。
func (s *Server) settingsFaviconUpload(c *gin.Context) {
	file, err := c.FormFile("favicon")
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("请选择图片文件"))
		return
	}
	src, err := file.Open()
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("读取文件失败"))
		return
	}
	defer src.Close()
	data, err := io.ReadAll(src)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("读取文件失败"))
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	var icoData []byte
	if ext == ".ico" {
		icoData = data // 已是 ico，直接使用
	} else {
		icoData, err = buildFaviconICO(data)
		if err != nil {
			c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape(err.Error()))
			return
		}
	}
	if err := os.MkdirAll(s.cfg.Upload.Dir, 0o755); err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("创建上传目录失败"))
		return
	}
	// 固定文件名 favicon.ico，覆盖旧图标
	if err := os.WriteFile(filepath.Join(s.cfg.Upload.Dir, "favicon.ico"), icoData, 0o644); err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("保存 favicon.ico 失败"))
		return
	}
	// 清理旧版自定义命名的图标文件
	if m, err := s.store.GetSettings(); err == nil {
		if old := m["favicon"]; strings.HasPrefix(old, "/uploads/") {
			base := filepath.Base(strings.SplitN(old, "?", 2)[0])
			if base != "favicon.ico" {
				_ = os.Remove(filepath.Join(s.cfg.Upload.Dir, base))
			}
		}
	}
	// 固定根路径 + 版本参数防缓存
	ver := strconv.FormatInt(time.Now().Unix(), 10)
	if err := s.store.SetSetting("favicon", "/favicon.ico?v="+ver); err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?err="+url.QueryEscape("保存设置失败"))
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings?saved=1")
}

// serveFavicon 根路径 /favicon.ico 提供图标（浏览器与搜索引擎默认请求位置）。
func (s *Server) serveFavicon(c *gin.Context) {
	path := filepath.Join(s.cfg.Upload.Dir, "favicon.ico")
	if _, err := os.Stat(path); err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(path)
}
