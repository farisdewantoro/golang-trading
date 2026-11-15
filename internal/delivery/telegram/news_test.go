package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseNewsArgs(t *testing.T) {
	valid, invalid := parseNewsArgs([]string{"bbca", "IDX:tlkm,ASII", "BBCA", "NASDAQ:AAPL", "bad code!"})
	assert.Equal(t, []string{"BBCA", "TLKM", "ASII"}, valid)
	assert.Equal(t, []string{"NASDAQ:AAPL", "bad code!"}, invalid)

	valid, invalid = parseNewsArgs(nil)
	assert.Empty(t, valid)
	assert.Empty(t, invalid)
}

func TestSplitNewsCallbackData(t *testing.T) {
	ex, code, err := splitNewsCallbackData("IDX:BBCA")
	assert.NoError(t, err)
	assert.Equal(t, "IDX", ex)
	assert.Equal(t, "BBCA", code)

	_, _, err = splitNewsCallbackData("BBCA")
	assert.Error(t, err)
}
