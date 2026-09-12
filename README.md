# blog-go

[简体中文](README.CN.md) | **English**

A lightweight personal blog built with **gin + sqlite**: write in Markdown on the admin side, clean and minimal front end, simple binary deployment.

## Screenshots

Home

![](/doc/home.png)

Post

![](/doc/post.png)

## Features

**Writing & Admin**
- editor.md editor with built-in image upload (stored via API, configurable directory)
- Draft / published status, optional publish date, editing, export raw `.md`
- Summaries: manual or auto-generated from content; existing posts are backfilled on startup
- Category management, site settings (title / ICP number / favicon / page size / SEO), password change

**Front End**
- Category navigation (A-Z, "Others" pinned last) + search + pagination, all combinable
- In-article table of contents with anchors, code highlighting (highlight.js · one-dark)
- Image viewer (Viewer.js: zoom / rotate / gallery), click-to-fullscreen videos, back-to-top

**Built-in Optimizations**
- Auto-generated `sitemap.xml`, `robots.txt` blocking admin routes, canonical / Open Graph / automatic descriptions on post pages
- Cookie sessions (HMAC-signed), bcrypt passwords
- Pure-Go sqlite driver (modernc) — no CGO, cross-compilation friendly

## Quick Start

Requires Go 1.26+.

```bash
git clone <your-repo-url> blog-go
cd blog-go
go run main.go
```

- Front end: http://localhost:8080
- Admin: http://localhost:8080/admin/login
- Default account: `admin` / `admin123` (seeded on first startup — change it immediately after login)

## Configuration (config.json)

| Field | Description | Default |
|---|---|---|
| `server.addr` | Listen address | `:8080` |
| `database.path` | sqlite database path | `data/blog.db` |
| `upload.dir` | Image upload directory | `data/uploads` |
| `auth.secret` | Session signing secret | **change this** |
| `auth.seed_username` / `auth.seed_password` | Initial admin | `admin` / `admin123` |
| `site.title` / `site.page_size` | Site title / default page size | `我的博客` / `20` |

> Site title, page size, etc. are stored in the `settings` table once saved; the database and upload directory can point anywhere

## Project Structure

```
main.go               entry point
internal/config       config loading
internal/store        sqlite, migrations, seed data
internal/web          routes & handlers
templates/            gin templates
static/               styles / editor.md / highlight.js / viewerjs / fonts
data/                 runtime data (git-ignored)
```

## Tech Stack

gin · modernc.org/sqlite · goldmark · editor.md · jQuery · highlight.js · Viewer.js
