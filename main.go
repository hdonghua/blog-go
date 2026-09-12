package main

import (
	"flag"
	"log"
	"strconv"

	"blog-go/internal/config"
	"blog-go/internal/store"
	"blog-go/internal/web"
)

func main() {
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	st, err := store.Open(cfg.Database.Path, cfg.Auth.SeedUsername, cfg.Auth.SeedPassword, map[string]string{
		"site_title": cfg.Site.Title,
		"icp":        "",
		"favicon":    "",
		"page_size":  strconv.Itoa(cfg.Site.PageSize),
	})
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer st.Close()
	if err := st.BackfillSummaries(); err != nil {
		log.Fatalf("回填文章摘要失败: %v", err)
	}

	srv := web.New(cfg, st)
	log.Printf("博客启动中: http://localhost%s (数据库: %s, 上传目录: %s)",
		cfg.Server.Addr, cfg.Database.Path, cfg.Upload.Dir)
	if err := srv.Router().Run(cfg.Server.Addr); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}
