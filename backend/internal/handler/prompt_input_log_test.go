package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPromptInputLogDedupesSameBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request := securityaudit.Request{Stage: "http"}
	body := []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)

	require.False(t, isPromptInputLoggedDuplicate(c, request, body))
	require.True(t, isPromptInputLoggedDuplicate(c, request, body))
	require.False(t, isPromptInputLoggedDuplicate(c, request, []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"other"}]}`)))
}

func TestLogPromptInputNeverPanics(t *testing.T) {
	dir := t.TempDir()
	logger.SetPromptInputDirOverride(dir)
	defer logger.SetPromptInputDirOverride("")
	defer logger.ClosePromptInputFile()

	require.NotPanics(t, func() {
		logPromptInput(securityaudit.Request{
			RequestID: "test", UserID: 7, APIKeyID: 9, Protocol: "openai_chat_completions",
			Model: "gpt-test", Stage: "http",
			Body: []byte(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`),
		})
	})
	require.NotPanics(t, func() {
		logPromptInput(securityaudit.Request{Body: []byte(`not-json`)})
	})
	require.NotPanics(t, func() {
		logPromptInput(securityaudit.Request{Body: []byte(`{"model":"x"}`)})
	})
}

func TestLogPromptInputWritesDailyFileWithLatestUserTurnOnly(t *testing.T) {
	dir := t.TempDir()
	logger.SetPromptInputDirOverride(dir)
	defer logger.SetPromptInputDirOverride("")
	defer logger.ClosePromptInputFile()

	logPromptInput(securityaudit.Request{
		RequestID: "req-daily", UserID: 7, APIKeyID: 9, Protocol: "openai_chat_completions",
		Model: "gpt-test", Stage: "http",
		Body: []byte(`{"model":"gpt-test","messages":[
			{"role":"system","content":"system instruction"},
			{"role":"user","content":"older user input"},
			{"role":"assistant","content":"older assistant output"},
			{"role":"user","content":"latest user input"}
		]}`),
	})
	logger.ClosePromptInputFile()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, strings.HasPrefix(entries[0].Name(), "prompt-input-"))
	require.True(t, strings.HasSuffix(entries[0].Name(), ".log"))

	raw, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	require.NoError(t, err)
	line := strings.TrimSpace(string(raw))
	require.Contains(t, line, `"latest user input"`)
	require.NotContains(t, line, "older user input")
	require.NotContains(t, line, "older assistant output")
	require.NotContains(t, line, "system instruction")
	require.Contains(t, line, `"user_id":7`)
}

func TestExtractLatestUserInputKeepsOnlyLatestUserTurn(t *testing.T) {
	snapshot, err := securityaudit.ExtractLatestUserInput(securityaudit.Request{
		Protocol: "openai_chat_completions",
		Body: []byte(`{"messages":[
			{"role":"system","content":"system instruction"},
			{"role":"user","content":"older user input"},
			{"role":"assistant","content":"older assistant output"},
			{"role":"tool","content":"tool payload"},
			{"role":"user","content":[{"type":"text","text":"latest first"},{"type":"text","text":"latest second"}]}
		]}`),
	})
	require.NoError(t, err)
	require.Equal(t, "latest first\n\nlatest second", snapshot.FullPrompt)
	require.Equal(t, 2, snapshot.MessageCount)

	_, err = securityaudit.ExtractLatestUserInput(securityaudit.Request{
		Protocol: "openai_chat_completions",
		Body:     []byte(`{"messages":[{"role":"system","content":"no user"},{"role":"assistant","content":"hi"}]}`),
	})
	require.ErrorIs(t, err, securityaudit.ErrNoPromptText)
}
