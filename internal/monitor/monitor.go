package monitor

import (
	"github.com/langchou/informer/db"
	mylog "github.com/langchou/informer/pkg/log"
	"github.com/langchou/informer/pkg/notifier"
)

type ForumMonitor interface {
	FetchPageContent() (string, error)
	ParseContent(content string) ([]Post, error)
	ProcessPosts(posts []Post) error
	MonitorPage()
}

func NewMonitor(forumName string, cookies string, userKeywords map[string][]string, notifier *notifier.DingTalkNotifier, database *db.Database, logger *mylog.Logger, waitTimeRange struct{ Min, Max int }) ForumMonitor {
	switch forumName {
	case "chiphell":
		return NewChiphellMonitor(forumName, cookies, userKeywords, notifier, database, logger, waitTimeRange)
	case "nga":
		// TODO
		return nil
	case "smzdm":
		// TODO
		return nil
	default:
		logger.Error("未知的论坛类型", "forumName", forumName)
		return nil
	}
}

type Post struct {
	Title    string
	Link     string
	Category string
}
