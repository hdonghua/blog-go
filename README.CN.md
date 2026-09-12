# blog-go

**简体中文** | [English](README.md)

基于 **gin + sqlite** 的轻量个人博客：后台 Markdown 写作，前台简洁风，二进制部署简单。

## 展示

首页

![](/doc/home.png)

详情

![](/doc/post.png)

## 功能

**写作与后台**
- editor.md 编辑器，图片直接上传（api 存储，目录可配置）
- 草稿 / 正式状态、文章日期可选、编辑、导出 `.md` 原文
- 摘要：可手填，留空自动从正文生成；老文章启动自动回填
- 分类管理、站点设置（标题 / 备案号 / favicon / 分页条数 / SEO）、修改密码

**前台**
- 分类导航（A-Z 排序，“其它”置底）+ 搜索 + 分页，条件可组合
- 文章目录（TOC）锚点定位、代码高亮（highlight.js · one-dark）
- 图片查看器（Viewer.js：缩放/旋转/多图切换）、视频点击全屏、回到顶部

**内置优化**
- `sitemap.xml` 自动生成、`robots.txt` 禁抓后台、文章页 canonical / Open Graph / 自动 description
- cookie 会话（HMAC 签名）、bcrypt 密码
- sqlite 使用 modernc 纯 Go 驱动，免 CGO，交叉编译友好

## 快速开始

要求 Go 1.26+。

```bash
git clone <你的仓库地址> blog-go
cd blog-go
go run main.go
```

- 前台：http://localhost:8080
- 后台：http://localhost:8080/admin/login
- 默认账号：`admin` / `admin123`（首次启动写入用户表，请登录后立即修改）

## 配置（config.json）

| 字段 | 说明 | 默认值 |
|---|---|---|
| `server.addr` | 监听地址 | `:8080` |
| `database.path` | sqlite 数据库路径 | `data/blog.db` |
| `upload.dir` | 图片上传目录 | `data/uploads` |
| `auth.secret` | 会话签名密钥 | **请修改** |
| `auth.seed_username` / `auth.seed_password` | 初始管理员 | `admin` / `admin123` |
| `site.title` / `site.page_size` | 站点标题 / 分页条数默认值 | `我的博客` / `20` |

> 站点标题、分页条数等保存进 `settings` 表后以数据库为准；数据库与上传目录可配置到任意路径

## 目录结构

```
main.go               入口
internal/config       配置加载
internal/store        sqlite、建表、种子数据
internal/web          路由与处理器
templates/            gin 模板
static/               样式 / editor.md / highlight.js / viewerjs / 字体
data/                 运行数据（已 git ignore）
```

## 技术栈

gin · modernc.org/sqlite · goldmark · editor.md · jQuery · highlight.js · Viewer.js
