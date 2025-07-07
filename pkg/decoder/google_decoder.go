package decoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang-trading/pkg/logger"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DecodeResult represents the result of decoding a Google News URL
type DecodeResult struct {
	Status     bool   `json:"status"`
	DecodedURL string `json:"decoded_url,omitempty"`
	Message    string `json:"message,omitempty"`
}

// Base64Result represents the result of extracting base64 string
type Base64Result struct {
	Status    bool   `json:"status"`
	Base64Str string `json:"base64_str,omitempty"`
	Message   string `json:"message,omitempty"`
}

// DecodingParamsResult represents the result of getting decoding parameters
type DecodingParamsResult struct {
	Status    bool   `json:"status"`
	Signature string `json:"signature,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Base64Str string `json:"base64_str,omitempty"`
	Message   string `json:"message,omitempty"`
}

// GoogleDecoder handles decoding of Google News URLs
type GoogleDecoder struct {
	Client *http.Client
	Logger *logger.Logger
}

// NewGoogleDecoder creates a new GoogleDecoder instance
func NewGoogleDecoder(logger *logger.Logger) *GoogleDecoder {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &GoogleDecoder{
		Client: client,
		Logger: logger,
	}
}

// getBase64Str extracts the base64 string from a Google News URL
func (g *GoogleDecoder) getBase64Str(sourceURL string) Base64Result {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		g.Logger.Error("Error parsing URL", logger.ErrorField(err))
		return Base64Result{
			Status:  false,
			Message: fmt.Sprintf("Error parsing URL: %v", err),
		}
	}

	path := strings.Split(parsed.Path, "/")
	if parsed.Hostname() == "news.google.com" && len(path) > 1 &&
		(path[len(path)-2] == "articles" || path[len(path)-2] == "read") {
		return Base64Result{
			Status:    true,
			Base64Str: path[len(path)-1],
		}
	}

	return Base64Result{
		Status:  false,
		Message: "Invalid Google News URL format",
	}
}

// extractSignatureAndTimestamp extracts signature and timestamp from HTML content
func extractSignatureAndTimestamp(body []byte) (string, string, error) {
	content := string(body)

	// Try multiple patterns for signature
	signaturePatterns := []*regexp.Regexp{
		regexp.MustCompile(`data-n-a-sg="([^"]*)"`),
		regexp.MustCompile(`data-n-a-sg='([^']*)'`),
		regexp.MustCompile(`data-n-a-sg=([^>\s]+)`),
	}

	var signature string
	for _, pattern := range signaturePatterns {
		matches := pattern.FindStringSubmatch(content)
		if len(matches) >= 2 {
			signature = matches[1]
			break
		}
	}

	if signature == "" {
		// Try alternative patterns
		altPatterns := []*regexp.Regexp{
			regexp.MustCompile(`"n-a-sg":"([^"]*)"`),
			regexp.MustCompile(`n-a-sg":"([^"]*)"`),
		}
		for _, pattern := range altPatterns {
			matches := pattern.FindStringSubmatch(content)
			if len(matches) >= 2 {
				signature = matches[1]
				break
			}
		}
	}

	if signature == "" {
		return "", "", errors.New("signature not found in HTML")
	}

	// Try multiple patterns for timestamp
	timestampPatterns := []*regexp.Regexp{
		regexp.MustCompile(`data-n-a-ts="([^"]*)"`),
		regexp.MustCompile(`data-n-a-ts='([^']*)'`),
		regexp.MustCompile(`data-n-a-ts=([^>\s]+)`),
	}

	var timestamp string
	for _, pattern := range timestampPatterns {
		matches := pattern.FindStringSubmatch(content)
		if len(matches) >= 2 {
			timestamp = matches[1]
			break
		}
	}

	if timestamp == "" {
		// Try alternative patterns
		altPatterns := []*regexp.Regexp{
			regexp.MustCompile(`"n-a-ts":"([^"]*)"`),
			regexp.MustCompile(`n-a-ts":"([^"]*)"`),
		}
		for _, pattern := range altPatterns {
			matches := pattern.FindStringSubmatch(content)
			if len(matches) >= 2 {
				timestamp = matches[1]
				break
			}
		}
	}

	if timestamp == "" {
		return "", "", errors.New("timestamp not found in HTML")
	}

	return signature, timestamp, nil
}

// getDecodingParams fetches signature and timestamp required for decoding
func (g *GoogleDecoder) getDecodingParams(base64Str string) DecodingParamsResult {
	// Try the first URL format
	urls := []string{
		fmt.Sprintf("https://news.google.com/rss/articles/%s", base64Str),
		fmt.Sprintf("https://news.google.com/articles/%s", base64Str),
	}

	for i, u := range urls {

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			g.Logger.Error("Error creating request for URL", logger.ErrorField(err), logger.IntField("url_index", i+1))
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

		resp, err := g.Client.Do(req)
		if err != nil {
			g.Logger.Error("Error making request for URL", logger.ErrorField(err), logger.IntField("url_index", i+1))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			g.Logger.Error("Non-OK status for URL", logger.IntField("url_index", i+1), logger.IntField("status_code", resp.StatusCode))
			continue
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			g.Logger.Error("Error reading body for URL", logger.ErrorField(err), logger.IntField("url_index", i+1))
			continue
		}

		signature, timestamp, err := extractSignatureAndTimestamp(body)
		if err == nil {
			return DecodingParamsResult{
				Status:    true,
				Signature: signature,
				Timestamp: timestamp,
				Base64Str: base64Str,
			}
		} else {
			g.Logger.Error("Error extracting signature/timestamp from URL", logger.ErrorField(err), logger.IntField("url_index", i+1))
		}
	}

	return DecodingParamsResult{
		Status:  false,
		Message: "Failed to fetch data attributes from Google News",
	}
}

// extractDecodedURL extracts the decoded URL from the parsed JSON response
func extractDecodedURL(parsedData []interface{}) (string, error) {
	if len(parsedData) == 0 {
		return "", errors.New("empty parsed data")
	}

	// Navigate through the JSON structure: parsedData[0][2]
	firstItem, ok := parsedData[0].([]interface{})
	if !ok {
		return "", errors.New("invalid data structure at index 0")
	}

	if len(firstItem) < 3 {
		return "", errors.New("insufficient data at index 0")
	}

	// Get the third element (index 2) which should be a JSON string
	jsonStr, ok := firstItem[2].(string)
	if !ok {
		return "", errors.New("expected string at index 2")
	}

	// Parse the JSON string
	var decodedData []interface{}
	if err := json.Unmarshal([]byte(jsonStr), &decodedData); err != nil {
		return "", fmt.Errorf("failed to parse JSON string: %v", err)
	}

	if len(decodedData) < 2 {
		return "", errors.New("insufficient decoded data")
	}

	// Get the second element (index 1) which should be the decoded URL
	decodedURL, ok := decodedData[1].(string)
	if !ok {
		return "", errors.New("expected string for decoded URL")
	}

	return decodedURL, nil
}

// decodeURL decodes the Google News URL using the signature and timestamp
func (g *GoogleDecoder) decodeURL(signature, timestamp, base64Str string) DecodeResult {
	apiURL := "https://news.google.com/_/DotsSplashUi/data/batchexecute"

	// Create the payload
	payload := []interface{}{
		"Fbv4je",
		fmt.Sprintf(`["garturlreq",[["X","X",["X","X"],null,null,1,1,"US:en",null,1,null,null,null,null,null,0,1],"X","X",1,[1,1,1],1,1,null,0,0,null,0],"%s",%s,"%s"]`, base64Str, timestamp, signature),
	}

	// Marshal the payload
	freq, err := json.Marshal([][]interface{}{{payload}})
	if err != nil {
		g.Logger.Error("Error marshaling payload", logger.ErrorField(err))
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("Error marshaling payload: %v", err),
		}
	}

	// Create form data
	data := url.Values{}
	data.Set("f.req", string(freq))

	// Create request
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		g.Logger.Error("Error creating request", logger.ErrorField(err))
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("Error creating request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")

	// Make request
	resp, err := g.Client.Do(req)
	if err != nil {
		g.Logger.Error("Error making request", logger.ErrorField(err))
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("Request error: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("HTTP error: %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("Error reading response: %v", err),
		}
	}

	responseStr := string(body)

	// Try different parsing approaches
	decodedURL, err := g.parseResponse(responseStr)
	if err != nil {
		return DecodeResult{
			Status:  false,
			Message: fmt.Sprintf("Error parsing response: %v", err),
		}
	}

	return DecodeResult{
		Status:     true,
		DecodedURL: decodedURL,
	}
}

// parseResponse tries different approaches to parse the Google News response
func (g *GoogleDecoder) parseResponse(responseStr string) (string, error) {
	// Method 1: Original approach - split by \n\n
	parts := strings.SplitN(responseStr, "\n\n", 2)
	if len(parts) >= 2 {
		jsonStr := parts[1]
		if len(jsonStr) >= 2 {
			jsonStr = jsonStr[:len(jsonStr)-2]
		}

		var parsedData []interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsedData); err == nil {
			if decodedURL, err := extractDecodedURL(parsedData); err == nil {
				return decodedURL, nil
			}
		}
	}

	// Method 2: Try to find JSON array directly
	// Look for patterns like )]}' followed by JSON
	if strings.Contains(responseStr, ")]}'") {
		parts := strings.Split(responseStr, ")]}'")
		if len(parts) >= 2 {
			jsonStr := strings.TrimSpace(parts[1])
			var parsedData []interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsedData); err == nil {
				if decodedURL, err := extractDecodedURL(parsedData); err == nil {
					return decodedURL, nil
				}
			}
		}
	}

	// Method 3: Try to extract URL directly using regex
	// Look for URLs in the response
	urlRegex := regexp.MustCompile(`https?://[^\s"']+`)
	matches := urlRegex.FindAllString(responseStr, -1)
	for _, match := range matches {
		// Filter out Google domains and common non-article URLs
		if !strings.Contains(match, "google.com") &&
			!strings.Contains(match, "gstatic.com") &&
			!strings.Contains(match, "googletagmanager.com") &&
			!strings.Contains(match, "doubleclick.net") {
			return match, nil
		}
	}

	// Method 4: Try parsing as a single JSON object
	var singleObj interface{}
	if err := json.Unmarshal([]byte(responseStr), &singleObj); err == nil {
		// Try to navigate through the object structure
		if decodedURL, err := g.extractURLFromObject(singleObj); err == nil {
			return decodedURL, nil
		}
	}

	return "", errors.New("could not parse response using any method")
}

// extractURLFromObject recursively searches for URLs in a JSON object
func (g *GoogleDecoder) extractURLFromObject(obj interface{}) (string, error) {
	switch v := obj.(type) {
	case string:
		// Check if it's a URL
		if strings.HasPrefix(v, "http") && !strings.Contains(v, "google.com") {
			return v, nil
		}
	case []interface{}:
		for _, item := range v {
			if url, err := g.extractURLFromObject(item); err == nil {
				return url, nil
			}
		}
	case map[string]interface{}:
		for _, value := range v {
			if url, err := g.extractURLFromObject(value); err == nil {
				return url, nil
			}
		}
	}
	return "", errors.New("no URL found in object")
}

// DecodeGoogleNewsURL decodes a Google News article URL into its original source URL
func (g *GoogleDecoder) DecodeGoogleNewsURL(sourceURL string, interval int) DecodeResult {
	// Extract base64 string
	base64Result := g.getBase64Str(sourceURL)
	if !base64Result.Status {
		return DecodeResult{
			Status:  false,
			Message: base64Result.Message,
		}
	}

	// Get decoding parameters
	paramsResult := g.getDecodingParams(base64Result.Base64Str)
	if !paramsResult.Status {
		return DecodeResult{
			Status:  false,
			Message: paramsResult.Message,
		}
	}

	// Add delay if specified
	if interval > 0 {
		time.Sleep(time.Duration(interval) * time.Second)
	}

	// Decode URL
	return g.decodeURL(paramsResult.Signature, paramsResult.Timestamp, paramsResult.Base64Str)
}
