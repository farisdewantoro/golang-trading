package repository

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoogleNewsRSS(t *testing.T) {
	body, err := os.ReadFile("testdata/google_news_bbca.xml")
	require.NoError(t, err)

	items, err := ParseGoogleNewsRSS(body)
	require.NoError(t, err)
	require.Len(t, items, 3, "item without link must be skipped")

	// newest first
	assert.Equal(t, "Asing Borong Saham BBCA & BBRI", items[0].Title, "source suffix stripped and entity decoded")
	assert.Equal(t, "Kontan", items[0].Source)
	assert.Equal(t, "CBMiBBB", items[0].GUID)
	assert.Equal(t, time.Date(2026, 11, 15, 1, 30, 0, 0, time.UTC), items[0].PublishedAt.UTC())

	assert.Equal(t, "Laba BBCA Naik 12% di Kuartal III", items[1].Title)
	assert.Equal(t, "Berita lama BBCA", items[2].Title)
}

func TestParseGoogleNewsRSSInvalid(t *testing.T) {
	_, err := ParseGoogleNewsRSS([]byte("not xml"))
	assert.Error(t, err)
}
