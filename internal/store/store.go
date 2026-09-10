package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound = errors.New("记录不存在")
	ErrExists   = errors.New("分类已存在")
)

type Store struct {
	DB *sql.DB
}

// Open 打开 sqlite 数据库，建表并初始化种子用户与默认配置。
func Open(dbPath string, seedUser, seedPassword string, defaultSettings map[string]string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	// modernc.org/sqlite 并发写会锁库，限制为单连接 + WAL 更稳
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("设置 sqlite 参数: %w", err)
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seedUser(seedUser, seedPassword); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seedSettings(defaultSettings); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS categories (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS posts (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			title       TEXT NOT NULL,
			category_id INTEGER NOT NULL DEFAULT 0,
			content     TEXT NOT NULL,
			status      INTEGER NOT NULL DEFAULT 1,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET DEFAULT
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB.Exec(stmt); err != nil {
			return fmt.Errorf("建表失败: %w\n%s", err, stmt)
		}
	}
	// 老库升级：为 posts 表补充 status 列
	var hasStatus int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('posts') WHERE name = 'status'`).Scan(&hasStatus); err != nil {
		return fmt.Errorf("检查 posts 表结构失败: %w", err)
	}
	if hasStatus == 0 {
		if _, err := s.DB.Exec(`ALTER TABLE posts ADD COLUMN status INTEGER NOT NULL DEFAULT 1`); err != nil {
			return fmt.Errorf("升级 posts 表失败: %w", err)
		}
	}
	return nil
}

// GetSettings 读取全部站点配置（网站标题、备案号等）。
func (s *Store) GetSettings() (map[string]string, error) {
	rows, err := s.DB.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

// SetSetting 保存单项站点配置。
func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// seedSettings 首次启动时把配置文件里的默认站点设置写入 settings 表。
func (s *Store) seedSettings(defaults map[string]string) error {
	for k, v := range defaults {
		var val string
		err := s.DB.QueryRow(`SELECT value FROM settings WHERE key = ?`, k).Scan(&val)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := s.DB.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, k, v); err != nil {
				return fmt.Errorf("写入默认配置: %w", err)
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

// seedUser 用户表为空时写入初始管理员。
func (s *Store) seedUser(username, password string) error {
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(hash), time.Now())
	if err != nil {
		return fmt.Errorf("写入种子用户: %w", err)
	}
	return nil
}

// GetUserByUsername 后台登录按用户名读取用户表。
func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.DB.QueryRow(`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.DB.Query(`SELECT id, name FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// CategoriesWithCount 返回全部分类及各自已发布文章数量，按名称 A-Z 排序，“其它”固定在最后。
func (s *Store) CategoriesWithCount() ([]CategoryCount, error) {
	rows, err := s.DB.Query(`SELECT c.id, c.name, COUNT(p.id)
		FROM categories c
		LEFT JOIN posts p ON p.category_id = c.id AND p.status = 1
		GROUP BY c.id, c.name
		ORDER BY CASE WHEN c.name = '其它' THEN 1 ELSE 0 END, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CategoryCount
	for rows.Next() {
		var c CategoryCount
		if err := rows.Scan(&c.ID, &c.Name, &c.Count); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) CreateCategory(name string) (int64, error) {
	res, err := s.DB.Exec(`INSERT INTO categories (name, created_at) VALUES (?, ?)`, name, time.Now())
	if err != nil {
		return 0, ErrExists
	}
	return res.LastInsertId()
}

// GetOrCreateCategoryByName 按名称查找分类，不存在则自动创建。
func (s *Store) GetOrCreateCategoryByName(name string) (int64, error) {
	var id int64
	err := s.DB.QueryRow(`SELECT id FROM categories WHERE name = ?`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO categories (name, created_at) VALUES (?, ?)`, name, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListPosts 前台文章列表，支持标题/内容模糊搜索与分页。
// postsWhere 拼接搜索/分类过滤条件（仅已发布）。
func postsWhere(query string, categoryID int64, args *[]any) string {
	conds := []string{"p.status = 1"}
	if query != "" {
		conds = append(conds, `(p.title LIKE ? ESCAPE '\' OR p.content LIKE ? ESCAPE '\')`)
		like := "%" + escapeLike(query) + "%"
		*args = append(*args, like, like)
	}
	if categoryID > 0 {
		conds = append(conds, "p.category_id = ?")
		*args = append(*args, categoryID)
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// ListPosts 前台文章列表（仅正式发布），支持搜索、分类过滤与分页。
func (s *Store) ListPosts(query string, categoryID int64, limit, offset int) ([]Post, error) {
	sqlStr := `SELECT p.id, p.title, p.category_id,
		COALESCE(c.name, ''), p.created_at
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id`
	args := []any{}
	sqlStr += postsWhere(query, categoryID, &args)
	sqlStr += ` ORDER BY p.created_at DESC, p.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.CategoryID, &p.CategoryName, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// CountPosts 前台文章总数（仅正式发布）。
func (s *Store) CountPosts(query string, categoryID int64) (int, error) {
	sqlStr := `SELECT COUNT(*) FROM posts p`
	args := []any{}
	sqlStr += postsWhere(query, categoryID, &args)
	var n int
	err := s.DB.QueryRow(sqlStr, args...).Scan(&n)
	return n, err
}

// ListAdminPosts 后台文章列表（含草稿）。
func (s *Store) ListAdminPosts(limit int) ([]Post, error) {
	rows, err := s.DB.Query(`SELECT p.id, p.title, p.category_id,
		COALESCE(c.name, ''), p.status, p.created_at
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		ORDER BY p.created_at DESC, p.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.CategoryID, &p.CategoryName, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// CountAdminPosts 后台文章总数（含草稿）。
func (s *Store) CountAdminPosts() (int, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&n)
	return n, err
}

// GetPost 读取单篇文章（markdown 原文）。
func (s *Store) GetPost(id int64) (*Post, error) {
	p := &Post{}
	err := s.DB.QueryRow(`SELECT p.id, p.title, p.category_id,
		COALESCE(c.name, ''), p.content, p.created_at, p.updated_at
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = ? AND p.status = 1`, id).
		Scan(&p.ID, &p.Title, &p.CategoryID, &p.CategoryName, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CreatePost 保存文章，content 为 markdown 原文，publishedAt 为文章日期。
func (s *Store) CreatePost(title string, categoryID int64, content string, publishedAt time.Time, status int) (int64, error) {
	now := time.Now()
	res, err := s.DB.Exec(`INSERT INTO posts (title, category_id, content, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, title, categoryID, content, status, publishedAt, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetPostForEdit 后台编辑用：按 ID 读取文章（含草稿）。
func (s *Store) GetPostForEdit(id int64) (*Post, error) {
	p := &Post{}
	err := s.DB.QueryRow(`SELECT p.id, p.title, p.category_id,
		COALESCE(c.name, ''), p.content, p.status, p.created_at, p.updated_at
		FROM posts p LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.id = ?`, id).
		Scan(&p.ID, &p.Title, &p.CategoryID, &p.CategoryName, &p.Content, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePost 更新文章（标题、分类、内容、日期、状态）。
func (s *Store) UpdatePost(id int64, title string, categoryID int64, content string, publishedAt time.Time, status int) error {
	res, err := s.DB.Exec(`UPDATE posts
		SET title = ?, category_id = ?, content = ?, status = ?, created_at = ?, updated_at = ?
		WHERE id = ?`,
		title, categoryID, content, status, publishedAt, time.Now(), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func escapeLike(s string) string {
	r := ""
	for _, c := range s {
		switch c {
		case '\\', '%', '_':
			r += "\\" + string(c)
		default:
			r += string(c)
		}
	}
	return r
}
