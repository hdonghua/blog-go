package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Server struct {
	Addr string `json:"addr"`
}

type Database struct {
	Path string `json:"path"`
}

type Upload struct {
	Dir string `json:"dir"`
}

type Auth struct {
	Secret       string `json:"secret"`
	CookieName   string `json:"cookie_name"`
	SessionHours int    `json:"session_hours"`
	SeedUsername string `json:"seed_username"`
	SeedPassword string `json:"seed_password"`
}

type Site struct {
	Title    string `json:"title"`
	PageSize int    `json:"page_size"`
}

type Config struct {
	Server   Server   `json:"server"`
	Database Database `json:"database"`
	Upload   Upload   `json:"upload"`
	Auth     Auth     `json:"auth"`
	Site     Site     `json:"site"`
}

// Load 读取配置文件，并为缺失项填充默认值。
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件: %w", err)
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
		}
	}
	fillDefaults(cfg)
	return cfg, nil
}

func fillDefaults(c *Config) {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Database.Path == "" {
		c.Database.Path = "data/blog.db"
	}
	if c.Upload.Dir == "" {
		c.Upload.Dir = "data/uploads"
	}
	if c.Auth.Secret == "" {
		c.Auth.Secret = "dev-secret"
	}
	if c.Auth.CookieName == "" {
		c.Auth.CookieName = "blog_session"
	}
	if c.Auth.SessionHours <= 0 {
		c.Auth.SessionHours = 72
	}
	if c.Auth.SeedUsername == "" {
		c.Auth.SeedUsername = "admin"
	}
	if c.Auth.SeedPassword == "" {
		c.Auth.SeedPassword = "admin123"
	}
	if c.Site.Title == "" {
		c.Site.Title = "我的博客"
	}
	if c.Site.PageSize <= 0 {
		c.Site.PageSize = 20
	}
}
