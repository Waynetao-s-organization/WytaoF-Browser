package browser

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
)

func openMigratedTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := database.NewDB(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("测试数据库迁移失败: %v", err)
	}
	return db.GetConn()
}

func TestSQLiteBookmarkDAOKeepsOpenScope(t *testing.T) {
	t.Parallel()

	dao := NewSQLiteBookmarkDAO(openMigratedTestDB(t))
	input := []config.BrowserBookmark{
		{Name: "ChatGPT", URL: "https://chatgpt.example/", OpenOnStart: true, OpenTags: []string{"Team", "样例"}, OpenGroupIds: []string{"group-1"}},
		{Name: "Google", URL: "https://google.example/"},
	}
	if err := dao.ReplaceAll(input); err != nil {
		t.Fatalf("ReplaceAll 返回错误: %v", err)
	}

	got, err := dao.List()
	if err != nil {
		t.Fatalf("List 返回错误: %v", err)
	}
	want := []config.BrowserBookmark{
		{Name: "ChatGPT", URL: "https://chatgpt.example/", OpenOnStart: true, OpenTags: []string{"Team", "样例"}, OpenGroupIds: []string{"group-1"}},
		{Name: "Google", URL: "https://google.example/", OpenTags: []string{}, OpenGroupIds: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("书签读回来的内容不对:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestSQLiteBookmarkDAOReadsRowsWrittenBeforeScopeColumns(t *testing.T) {
	t.Parallel()

	conn := openMigratedTestDB(t)
	if _, err := conn.Exec(`INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order) VALUES ('旧书签', 'https://old.example/', 1, 0)`); err != nil {
		t.Fatalf("写入旧格式书签失败: %v", err)
	}

	got, err := NewSQLiteBookmarkDAO(conn).List()
	if err != nil {
		t.Fatalf("List 返回错误: %v", err)
	}
	want := []config.BrowserBookmark{
		{Name: "旧书签", URL: "https://old.example/", OpenOnStart: true, OpenTags: []string{}, OpenGroupIds: []string{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("旧格式书签应当按“对所有实例打开”读出来:\n got=%+v\nwant=%+v", got, want)
	}
}
