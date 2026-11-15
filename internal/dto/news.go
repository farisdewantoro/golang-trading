package dto

import (
	"encoding/xml"
	"time"
)

// NewsItem is a single normalized news article.
type NewsItem struct {
	Title       string
	Link        string
	GUID        string
	Source      string
	PublishedAt time.Time
}

// SearchNewsParam describes a news search request.
type SearchNewsParam struct {
	Query    string
	Language string // e.g. "id"
	Country  string // e.g. "ID"
}

// GoogleNewsRSS maps the Google News RSS feed.
type GoogleNewsRSS struct {
	XMLName xml.Name          `xml:"rss"`
	Channel GoogleNewsChannel `xml:"channel"`
}

type GoogleNewsChannel struct {
	Title string           `xml:"title"`
	Items []GoogleNewsItem `xml:"item"`
}

type GoogleNewsItem struct {
	Title   string           `xml:"title"`
	Link    string           `xml:"link"`
	GUID    string           `xml:"guid"`
	PubDate string           `xml:"pubDate"`
	Source  GoogleNewsSource `xml:"source"`
}

type GoogleNewsSource struct {
	URL  string `xml:"url,attr"`
	Name string `xml:",chardata"`
}
