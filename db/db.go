// db/db.go
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/langchou/informer/pkg/log"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	DB  *sql.DB
	Log *log.Logger // 引入 logger 实例
}

// InitDB 初始化数据库
func InitDB(filepath string, logger *log.Logger) (*Database, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return &Database{DB: db, Log: logger}, nil
}

// CreateTableIfNotExists 创建一个用于存储帖子的表
func (d *Database) CreateTableIfNotExists() error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT NOT NULL UNIQUE,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := d.DB.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("无法创建表 posts: %v", err)
	}
	return nil
}

// IsNewPost 检查帖子哈希是否已经存在
func (d *Database) IsNewPost(hash string) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM posts WHERE hash = ?)`
	err := d.DB.QueryRow(query, hash).Scan(&exists)
	if err != nil {
		d.Log.Error("数据库查询错误", "error", err)
		return false
	}
	return !exists
}

// StorePostHash 存储新的帖子哈希
func (d *Database) StorePostHash(hash string) {
	insertQuery := `INSERT INTO posts (hash) VALUES (?)`
	_, err := d.DB.Exec(insertQuery, hash)
	if err != nil {
		d.Log.Error("无法存储帖子哈希", "error", err)
	}
}

// CleanUpOldPosts 清理数据库中过期的帖子记录
func (d *Database) CleanUpOldPosts(duration time.Duration) {
	deleteQuery := `DELETE FROM posts WHERE timestamp < datetime('now', ?)`
	_, err := d.DB.Exec(deleteQuery, fmt.Sprintf("-%d seconds", int(duration.Seconds())))
	if err != nil {
		d.Log.Error("无法清理旧帖子记录", "error", err)
	}
}
