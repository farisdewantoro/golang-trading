package decoder

import (
	"golang-trading/config"
	"golang-trading/pkg/logger"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleDecoder_DecodeGoogleNewsURL(t *testing.T) {
	type fields struct {
		Client *http.Client
	}
	type args struct {
		sourceURL string
		interval  int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   DecodeResult
	}{
		{
			name: "Test with invalid URL",
			fields: fields{
				Client: &http.Client{},
			},
			args: args{
				sourceURL: "https://news.google.com/rss/articles/INVALID?oc=5",
				interval:  1,
			},
			want: DecodeResult{
				Status: false,
			},
		},
		{
			name: "Test with valid URL cnbcindonesia",
			fields: fields{
				Client: &http.Client{},
			},
			args: args{
				sourceURL: "https://news.google.com/rss/articles/CBMivAFBVV95cUxPby01ek9QZnBZbENDLU9WVzBvaVRLUWVVWmZ4bXpQeGkyOFVRNFhRSlh6eTc1ZmxBbVZfclRfbUdyWmw3WGNyMmtnLTBIb1FkYzNYWUR0aFRnWDBiVTYwUjVTR1RxdWNJTGtOYkRrOEh3WF80Zkd2YlN5d1RSd2Y5bEoxOXlXQl9SWXpoT0o4S09uOHlTV1E2RklxTm1lVDJLVl9POHlZX19jT3huX3B0alotSUtHTFZXcEhHNA?oc=5",
				interval:  1,
			},
			want: DecodeResult{
				DecodedURL: "https://www.cnbcindonesia.com/research/20250613072333-128-640644/lengkap-ini-besaran-dividen-raksasa-ri-ptba-sampai-antm",
				Status:     true,
			},
		},
		{
			name: "Test with valid URL market.bisnis.com",
			fields: fields{
				Client: &http.Client{},
			},
			args: args{
				sourceURL: "https://news.google.com/rss/articles/CBMisAFBVV95cUxPejVkcGFOR1RKUmJOdnVMS2FRVDRRWmt2M0VBSEVWNUFFdVhHSW9Oc2VObGItSjdaRnl5UThDMVVRNHd6ZEotMEVtYkZUMk1mWFJCR2dETC1NNG4wX3RRMUJ4MU9OYzlzdjQxdVVBXzZ3cXFNYWNnUkxqVGVudklOeTh4a1daWkZmVDJxSS1qbWdmT19aaTFTdnNkblp2MVp6bV9DUGxOQmlEcEVNcmlqRw?oc=5",
				interval:  1,
			},
			want: DecodeResult{
				DecodedURL: "https://market.bisnis.com/read/20250613/7/1884847/konflik-israel-iran-bikin-saham-antm-arci-hingga-psab-melesat",
				Status:     true,
			},
		},
		{
			name: "Test with valid URL investor.id",
			fields: fields{
				Client: &http.Client{},
			},
			args: args{
				sourceURL: "https://news.google.com/rss/articles/CBMiuwFBVV95cUxPR3F6NWtRZE1CUzNxVGhkTUV4dXRUUXZNWXNpemJMMWE2SjJfRVRNWVU2azBZRlFPcHVtNUM4RTJTWUdUT0NwUUpzWDlxNUlUS3RPWUN2bmdrTTVITVlsX0QxX1AyQVh0ejQyRFBKRlpRcjR3RXp3alg1bnRJUVJrbjdJWmt1RVdFZVJUelY0bDM2a0h2WnVUSDgtNXhTZWNGUWszVkpGMzk4R18yOVUzUXZrRGpwd2JxODBj?oc=5",
				interval:  1,
			},
			want: DecodeResult{
				DecodedURL: "https://investor.id/market/400394/berita-populer-harga-emas-perhiasan-dan-antam-antm-kian-panas-hingga-nasib-saham-adro",
				Status:     true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			log, _ := logger.New(cfg)
			g := &GoogleDecoder{
				Client: tt.fields.Client,
				Logger: log,
			}
			got := g.DecodeGoogleNewsURL(tt.args.sourceURL, tt.args.interval)
			assert.Equal(t, tt.want.Status, got.Status, "Status mismatch")
			assert.Equal(t, tt.want.DecodedURL, got.DecodedURL, "DecodedURL mismatch")
		})
	}
}
