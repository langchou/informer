package monitor

type ForumMonitor interface {
	FetchPageContent() (string, error)
	ParseContent(content string) ([]Post, error)
	ProcessPosts(posts []Post) error
}

type Post struct {
	Title    string
	Link     string
	Category string
}
