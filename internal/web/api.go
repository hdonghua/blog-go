package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var allowedImageExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".ico": true,
}

// saveUploadedImage 校验并保存上传图片，返回可访问的 URL 路径。
func (s *Server) saveUploadedImage(c *gin.Context, field string) (string, error) {
	file, err := c.FormFile(field)
	if err != nil {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedImageExt[ext] {
		return "", fmt.Errorf("仅支持 ico/jpg/jpeg/png/gif/webp 图片")
	}
	if err := os.MkdirAll(s.cfg.Upload.Dir, 0o755); err != nil {
		return "", fmt.Errorf("创建上传目录失败: %w", err)
	}
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	name := hex.EncodeToString(buf) + ext
	dst := filepath.Join(s.cfg.Upload.Dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}
	return "/uploads/" + name, nil
}

// uploadImage editor.md 的图片上传接口，保存到可配置的上传目录。
func (s *Server) uploadImage(c *gin.Context) {
	url, err := s.saveUploadedImage(c, "editormd-image-file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": 0, "message": err.Error()})
		return
	}
	// editor.md 约定的返回格式
	c.JSON(http.StatusOK, gin.H{"success": 1, "url": url, "message": "上传成功"})
}
