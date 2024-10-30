package db

import (
	"database/sql"
	"fmt"
	mylog "github.com/langchou/informer/pkg/log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	DB *sql.DB
}

func InitDB(filepath string) (*Database, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return &Database{DB: db}, nil
}

func (d *Database) CreateTableIfNotExists(forum string) error {
	tableName := fmt.Sprintf("%s_posts", forum)
	createTableQuery := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		post_id TEXT NOT NULL UNIQUE,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`, tableName)

	_, err := d.DB.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("无法创建表 %s: %v", tableName, err)
	}
	return nil
}

func (d *Database) IsNewPost(forum, postID string) bool {
	tableName := fmt.Sprintf("%s_posts", forum)
	var exists bool
	query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE post_id = ?)`, tableName)
	err := d.DB.QueryRow(query, postID).Scan(&exists)
	if err != nil {
		mylog.Error("数据库查询错误", "error", err)
		return false
	}
	return !exists
}

func (d *Database) StorePostID(forum, postID string) {
	tableName := fmt.Sprintf("%s_posts", forum)
	insertQuery := fmt.Sprintf(`INSERT INTO %s (post_id) VALUES (?)`, tableName)
	_, err := d.DB.Exec(insertQuery, postID)
	if err != nil {
		mylog.Error("无法存储帖子ID", "error", err)
	}
}

func (d *Database) CleanUpOldPosts(forum string, duration time.Duration) {
	tableName := fmt.Sprintf("%s_posts", forum)
	deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE timestamp < datetime('now', ?)`, tableName)
	_, err := d.DB.Exec(deleteQuery, fmt.Sprintf("-%d seconds", int(duration.Seconds())))
	if err != nil {
		mylog.Error("无法清理旧帖子记录", "error", err)
	}
}
