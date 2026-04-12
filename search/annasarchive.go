package search

import (
	"fmt"
	"net/url"
)

// AnnaArchive 基于 Anna's Archive 的电子书搜索源
type AnnaArchive struct{}

func NewAnnaArchive() *AnnaArchive {
	return &AnnaArchive{}
}

func (a *AnnaArchive) Name() string {
	return "Anna's Archive"
}

func (a *AnnaArchive) SearchBooks(keyword string) ([]BookResult, error) {
	q := url.QueryEscape(keyword)
	return []BookResult{
		{
			Title:     fmt.Sprintf("%s — epub (中文优先)", keyword),
			Format:    "EPUB",
			Source:    a.Name(),
			DirectURL: fmt.Sprintf("https://annas-archive.org/search?q=%s&ext=epub&lang=zh", q),
		},
		{
			Title:     fmt.Sprintf("%s — 全格式", keyword),
			Format:    "EPUB/PDF/MOBI",
			Source:    a.Name(),
			DirectURL: fmt.Sprintf("https://annas-archive.org/search?q=%s", q),
		},
	}, nil
}
