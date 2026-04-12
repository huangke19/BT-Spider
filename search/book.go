package search

// BookResult 电子书搜索结果
type BookResult struct {
	Title     string // 书名
	Author    string // 作者
	Format    string // 格式，如 epub/pdf/mobi
	Size      string // 文件大小
	Source    string // 来源站点
	DirectURL string // 下载页面 URL（非直链，需浏览器访问）
	Language  string // 语言
	Year      string // 出版年份
}

// BookProvider 电子书搜索源接口
type BookProvider interface {
	Name() string
	SearchBooks(keyword string) ([]BookResult, error)
}

// SearchBooks 使用所有电子书源搜索，合并结果
func SearchBooks(keyword string, providers []BookProvider) ([]BookResult, error) {
	var all []BookResult
	for _, p := range providers {
		results, err := p.SearchBooks(keyword)
		if err != nil {
			continue
		}
		all = append(all, results...)
	}
	return all, nil
}
