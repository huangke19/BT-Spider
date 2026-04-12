package search

import (
	"fmt"
	"net/url"
)

// ZLibrary 基于 Z-Library 的电子书搜索源
type ZLibrary struct{}

func NewZLibrary() *ZLibrary {
	return &ZLibrary{}
}

func (z *ZLibrary) Name() string {
	return "Z-Library"
}

func (z *ZLibrary) SearchBooks(keyword string) ([]BookResult, error) {
	q := url.PathEscape(keyword)
	return []BookResult{
		{
			Title:     fmt.Sprintf("%s — epub (中文)", keyword),
			Format:    "EPUB",
			Source:    z.Name(),
			DirectURL: fmt.Sprintf("https://zh.z-library.sk/s/%s/?extensions[]=epub", q),
		},
		{
			Title:     fmt.Sprintf("%s — 全格式", keyword),
			Format:    "EPUB/PDF/MOBI",
			Source:    z.Name(),
			DirectURL: fmt.Sprintf("https://zh.z-library.sk/s/%s/", q),
		},
	}, nil
}
