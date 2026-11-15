package strategy

import (
	"golang-trading/internal/dto"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildNewsQuery(t *testing.T) {
	assert.Equal(t, `"BBCA" saham`, BuildNewsQuery("", "bbca"))
	assert.Equal(t, `"BBCA" saham`, BuildNewsQuery("no placeholder", "BBCA"))
	assert.Equal(t, "TLKM emiten", BuildNewsQuery("%s emiten", "tlkm"))
}

func TestFilterRecentNews(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	items := []dto.NewsItem{
		{Title: "old", PublishedAt: now.Add(-48 * time.Hour)},
		{Title: "a", PublishedAt: now.Add(-3 * time.Hour)},
		{Title: "no-date"},
		{Title: "b", PublishedAt: now.Add(-1 * time.Hour)},
		{Title: "c", PublishedAt: now.Add(-2 * time.Hour)},
	}

	got := FilterRecentNews(items, now, 24*time.Hour, 2)
	assert.Len(t, got, 2)
	assert.Equal(t, "b", got[0].Title)
	assert.Equal(t, "c", got[1].Title)

	all := FilterRecentNews(items, now, 24*time.Hour, 0)
	assert.Len(t, all, 3)
}

func TestNewsHash(t *testing.T) {
	item := dto.NewsItem{GUID: "g1", Link: "https://x/1"}
	h1 := NewsHash("IDX", "BBCA", item)
	assert.Len(t, h1, 64)
	assert.Equal(t, h1, NewsHash("idx", "bbca", item), "case-insensitive")
	assert.NotEqual(t, h1, NewsHash("IDX", "BBRI", item), "same article for another stock is separate")
	assert.NotEqual(t, h1, NewsHash("IDX", "BBCA", dto.NewsItem{GUID: "g2"}))
	// falls back to link when GUID missing
	assert.Equal(t, NewsHash("IDX", "BBCA", dto.NewsItem{Link: "l"}), NewsHash("IDX", "BBCA", dto.NewsItem{Link: "l"}))
}
