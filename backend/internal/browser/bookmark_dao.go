package browser

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ant-chrome/backend/internal/config"
)

// BookmarkDAO 默认书签持久化接口
type BookmarkDAO interface {
	List() ([]config.BrowserBookmark, error)
	ReplaceAll(bookmarks []config.BrowserBookmark) error
}

// SQLiteBookmarkDAO 基于 SQLite 的 BookmarkDAO 实现
type SQLiteBookmarkDAO struct {
	db *sql.DB
}

// NewSQLiteBookmarkDAO 创建 SQLiteBookmarkDAO
func NewSQLiteBookmarkDAO(db *sql.DB) *SQLiteBookmarkDAO {
	return &SQLiteBookmarkDAO{db: db}
}

// List 查询所有默认书签，按 sort_order 升序
func (d *SQLiteBookmarkDAO) List() ([]config.BrowserBookmark, error) {
	rows, err := d.db.Query(`
		SELECT name, url, COALESCE(open_on_start, 0), COALESCE(open_tags, '[]'), COALESCE(open_group_ids, '[]')
		FROM browser_bookmarks ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询书签列表失败: %w", err)
	}
	defer rows.Close()

	var list []config.BrowserBookmark
	for rows.Next() {
		var b config.BrowserBookmark
		var openOnStart int
		var openTags, openGroupIds string
		if err := rows.Scan(&b.Name, &b.URL, &openOnStart, &openTags, &openGroupIds); err != nil {
			return nil, fmt.Errorf("读取书签行失败: %w", err)
		}
		b.OpenOnStart = openOnStart != 0
		b.OpenTags = decodeBookmarkScope(openTags)
		b.OpenGroupIds = decodeBookmarkScope(openGroupIds)
		list = append(list, b)
	}
	return list, rows.Err()
}

// ReplaceAll 原子替换全部书签（事务保证）
func (d *SQLiteBookmarkDAO) ReplaceAll(bookmarks []config.BrowserBookmark) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM browser_bookmarks`); err != nil {
		return fmt.Errorf("清空书签失败: %w", err)
	}
	for i, b := range bookmarks {
		if b.Name == "" || b.URL == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO browser_bookmarks (name, url, open_on_start, open_tags, open_group_ids, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
			b.Name, b.URL, boolToInt(b.OpenOnStart), encodeBookmarkScope(b.OpenTags), encodeBookmarkScope(b.OpenGroupIds), i,
		); err != nil {
			return fmt.Errorf("插入书签失败: %w", err)
		}
	}
	return tx.Commit()
}

// NormalizeBookmarkScope 去掉空白项和重复项，保证返回非 nil 切片
func NormalizeBookmarkScope(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func decodeBookmarkScope(raw string) []string {
	var items []string
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &items)
	}
	return NormalizeBookmarkScope(items)
}

func encodeBookmarkScope(items []string) string {
	data, err := json.Marshal(NormalizeBookmarkScope(items))
	if err != nil {
		return "[]"
	}
	return string(data)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
