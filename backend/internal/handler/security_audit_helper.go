package handler

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const securityAuditCompletedContextKey = "sub2api.security_audit.completed"
const securityAuditWSTurnContextKey = "sub2api.security_audit.ws_turn"
const securityAuditWSDedupeContextKey = "sub2api.security_audit.ws_dedupe"

type securityAuditWSDedupeEntry struct {
	stage    string
	turn     int
	bodyHash [sha256.Size]byte
	decision securityaudit.Decision
}

// cachesSecurityAuditCompletion reports whether a successful audit may be
// reused for the rest of the gin request. WebSocket turns share one Context
// across many response.create frames and must be audited independently.
func cachesSecurityAuditCompletion(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "", "http":
		return true
	default:
		return false
	}
}

func isSecurityAuditWebSocketStage(stage string) bool {
	switch strings.TrimSpace(stage) {
	case "first_turn", "subsequent_turn":
		return true
	default:
		return false
	}
}

func (h *GatewayHandler) checkSecurityAudit(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol, model string, body []byte) *securityaudit.Decision {
	if h == nil {
		return nil
	}
	return runSecurityAudit(c, reqLog, h.securityAuditCoordinator, h.contentModerationService, apiKey, subject, protocol, model, body, "http")
}

func (h *OpenAIGatewayHandler) checkSecurityAudit(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol, model string, body []byte) *securityaudit.Decision {
	if h == nil {
		return nil
	}
	return runSecurityAudit(c, reqLog, h.securityAuditCoordinator, h.contentModerationService, apiKey, subject, protocol, model, body, "http")
}

func (h *OpenAIGatewayHandler) checkSecurityAuditStage(c *gin.Context, reqLog *zap.Logger, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol, model string, body []byte, stage string) *securityaudit.Decision {
	if h == nil {
		return nil
	}
	return runSecurityAudit(c, reqLog, h.securityAuditCoordinator, h.contentModerationService, apiKey, subject, protocol, model, body, stage)
}

func runSecurityAudit(c *gin.Context, reqLog *zap.Logger, coordinator *securityaudit.Coordinator, legacy *service.ContentModerationService, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol, model string, body []byte, stage string) *securityaudit.Decision {
	if c == nil || c.Request == nil {
		return nil
	}
	cacheCompletion := cachesSecurityAuditCompletion(stage)
	if cacheCompletion {
		if completed, exists := c.Get(securityAuditCompletedContextKey); exists && completed == true {
			return nil
		}
	}
	request := buildSecurityAuditRequest(c, apiKey, subject, protocol, model, body, stage)
	// Prompt-input file log: input only, never the upstream return.
	// Independent of audit config; original prompt-audit flow is untouched.
	// Placed after the HTTP completion-cache check above so one HTTP request
	// logs once, but before coordinator branching so legacy/coordinator-off
	// traffic is still logged.
	if !isPromptInputLoggedDuplicate(c, request, body) {
		logPromptInput(request)
	}
	if coordinator == nil {
		legacyDecision := runContentModeration(c, reqLog, legacy, apiKey, subject, protocol, model, body)
		if legacyDecision == nil {
			return nil
		}
		decision := securityaudit.Decision{Kind: securityaudit.DecisionAllow, HTTPStatus: http.StatusOK, AllowNextStage: true}
		decision.Legacy = &securityaudit.LegacyDecision{
			Allowed: legacyDecision.Allowed, Blocked: legacyDecision.Blocked, Flagged: legacyDecision.Flagged,
			Message: legacyDecision.Message, StatusCode: legacyDecision.StatusCode,
			ErrorCode: "content_policy_violation", Action: legacyDecision.Action,
		}
		if legacyDecision.Blocked {
			decision.Kind, decision.HTTPStatus, decision.ErrorCode, decision.ClientMessage, decision.AllowNextStage = securityaudit.DecisionBlock, contentModerationStatus(legacyDecision), "content_policy_violation", legacyDecision.Message, false
		}
		if decision.AllowNextStage && cacheCompletion {
			c.Set(securityAuditCompletedContextKey, true)
		}
		return &decision
	}
	if isSecurityAuditWebSocketStage(request.Stage) {
		if turnNo, ok := securityAuditWSTurn(c); ok {
			bodyHash := sha256.Sum256(body)
			if cached, exists := c.Get(securityAuditWSDedupeContextKey); exists {
				if entry, ok := cached.(securityAuditWSDedupeEntry); ok &&
					entry.stage == request.Stage && entry.turn == turnNo && entry.bodyHash == bodyHash {
					decision := entry.decision
					logSecurityAuditDone(reqLog, request, decision, true)
					return &decision
				}
			}
			logSecurityAuditStart(reqLog, request, len(body), false)
			decision := coordinator.Check(c.Request.Context(), request)
			if decision.Kind == securityaudit.DecisionAllow {
				c.Set(securityAuditWSDedupeContextKey, securityAuditWSDedupeEntry{
					stage: request.Stage, turn: turnNo, bodyHash: bodyHash, decision: decision,
				})
			}
			logSecurityAuditDone(reqLog, request, decision, false)
			return &decision
		}
	}
	logSecurityAuditStart(reqLog, request, len(body), false)
	decision := coordinator.Check(c.Request.Context(), request)
	if decision.AllowNextStage && cacheCompletion {
		c.Set(securityAuditCompletedContextKey, true)
	}
	logSecurityAuditDone(reqLog, request, decision, false)
	return &decision
}

const promptInputLoggedContextKey = "sub2api.prompt_input.logged"

// promptInputLoggedEntry dedupes the input-only file log per gin request or
// per WebSocket turn, mirroring the audit completion/WS caches above.
type promptInputLoggedEntry struct {
	stage    string
	turn     int
	hasTurn  bool
	bodyHash [sha256.Size]byte
}

// isPromptInputLoggedDuplicate reports whether this exact input was already
// logged, and records it when first seen.
func isPromptInputLoggedDuplicate(c *gin.Context, request securityaudit.Request, body []byte) bool {
	if c == nil {
		return false
	}
	entry := promptInputLoggedEntry{stage: request.Stage, bodyHash: sha256.Sum256(body)}
	if turn, ok := securityAuditWSTurn(c); ok {
		entry.turn, entry.hasTurn = turn, true
	}
	if cached, exists := c.Get(promptInputLoggedContextKey); exists {
		if prev, ok := cached.(promptInputLoggedEntry); ok && prev == entry {
			return true
		}
	}
	c.Set(promptInputLoggedContextKey, entry)
	return false
}

// logPromptInput writes the input-only prompt log to the dedicated daily
// file (<logdir>/prompt-input-YYYY-MM-DD.log, one JSON object per line).
// Only the latest user turn is recorded: no system instructions, no history,
// no assistant/tool output. It never touches the upstream response, never
// writes prompt_audit tables, and never enters the ops DB sink.
func logPromptInput(request securityaudit.Request) {
	snapshot, err := securityaudit.ExtractLatestUserInput(request)
	if err != nil {
		if errors.Is(err, securityaudit.ErrNoPromptText) {
			return
		}
		logger.L().Warn("prompt_input.extract_failed",
			zap.String("component", "prompt_input"),
			zap.String("request_id", request.RequestID),
			zap.Int64("user_id", request.UserID),
			zap.Int64("api_key_id", request.APIKeyID),
			zap.Bool(logger.OpsSystemLogSkipField, true),
		)
		return
	}
	record := map[string]any{
		"time":          time.Now().UTC().Format(time.RFC3339Nano),
		"component":     "prompt_input",
		"request_id":    request.RequestID,
		"user_id":       request.UserID,
		"username":      request.Username,
		"api_key_id":    request.APIKeyID,
		"api_key_name":  request.APIKeyName,
		"group_id":      request.GroupID,
		"group_name":    request.GroupName,
		"endpoint":      request.Endpoint,
		"provider":      request.Provider,
		"protocol":      request.Protocol,
		"model":         request.Model,
		"stage":         snapshot.Stage,
		"prompt_hash":   snapshot.PromptHash,
		"prompt_length": snapshot.PromptLength,
		"message_count": snapshot.MessageCount,
		"prompt":        snapshot.FullPrompt,
	}
	line, err := json.Marshal(record)
	if err != nil {
		logger.L().Warn("prompt_input.encode_failed",
			zap.String("component", "prompt_input"),
			zap.String("request_id", request.RequestID),
			zap.Bool(logger.OpsSystemLogSkipField, true),
		)
		return
	}
	if err := logger.AppendPromptInputLine(line); err != nil {
		logger.L().Warn("prompt_input.write_failed",
			zap.String("component", "prompt_input"),
			zap.String("request_id", request.RequestID),
			zap.String("file", logger.PromptInputFilename()),
			zap.Bool(logger.OpsSystemLogSkipField, true),
		)
	}
}

func logSecurityAuditStart(reqLog *zap.Logger, request securityaudit.Request, bodyBytes int, cached bool) {
	if reqLog == nil {
		return
	}
	reqLog.Info("security_audit.gateway_check_start",
		zap.String("request_id", request.RequestID), zap.Int64("user_id", request.UserID),
		zap.Int64("api_key_id", request.APIKeyID), zap.Int64p("group_id", request.GroupID),
		zap.String("endpoint", request.Endpoint), zap.String("provider", request.Provider),
		zap.String("protocol", request.Protocol), zap.String("model", request.Model), zap.String("stage", request.Stage),
		zap.Int("body_bytes", bodyBytes), zap.Bool("cached", cached))
}

func logSecurityAuditDone(reqLog *zap.Logger, request securityaudit.Request, decision securityaudit.Decision, cached bool) {
	if reqLog == nil {
		return
	}
	reqLog.Info("security_audit.gateway_check_done",
		zap.String("request_id", request.RequestID), zap.String("decision", string(decision.Kind)),
		zap.String("error_code", decision.ErrorCode), zap.Bool("allow_next_stage", decision.AllowNextStage),
		zap.String("stage", request.Stage), zap.Bool("cached", cached))
}

func securityAuditWSTurn(c *gin.Context) (int, bool) {
	turn, exists := c.Get(securityAuditWSTurnContextKey)
	if !exists {
		return 0, false
	}
	turnNo, ok := turn.(int)
	return turnNo, ok
}

func buildSecurityAuditRequest(c *gin.Context, apiKey *service.APIKey, subject middleware2.AuthSubject, protocol, model string, body []byte, stage string) securityaudit.Request {
	legacy := buildContentModerationInput(c, apiKey, subject, protocol, model, body)
	request := securityaudit.Request{
		RequestID: legacy.RequestID, UserID: legacy.UserID, UserEmail: legacy.UserEmail,
		APIKeyID: legacy.APIKeyID, APIKeyName: legacy.APIKeyName, GroupID: cloneSecurityAuditGroupID(legacy.GroupID),
		GroupName: legacy.GroupName, Provider: legacy.Provider, Endpoint: legacy.Endpoint,
		Protocol: legacy.Protocol, Model: legacy.Model, Body: body, Stage: strings.TrimSpace(stage),
	}
	if apiKey != nil && apiKey.User != nil {
		request.Username = apiKey.User.Username
		if request.UserEmail == "" {
			request.UserEmail = apiKey.User.Email
		}
	}
	if request.Stage == "" {
		request.Stage = "http"
	}
	return request
}

func securityAuditStatus(decision *securityaudit.Decision) int {
	if decision == nil || decision.HTTPStatus < 400 || decision.HTTPStatus > 599 {
		return http.StatusForbidden
	}
	return decision.HTTPStatus
}

func securityAuditErrorCode(decision *securityaudit.Decision) string {
	if decision == nil || strings.TrimSpace(decision.ErrorCode) == "" {
		return "content_policy_violation"
	}
	return decision.ErrorCode
}

func securityAuditMessage(decision *securityaudit.Decision) string {
	if decision == nil {
		return "Request blocked by content policy"
	}
	if decision.Legacy != nil && decision.Legacy.Blocked && strings.TrimSpace(decision.Legacy.Message) != "" {
		return decision.Legacy.Message
	}
	if strings.TrimSpace(decision.ClientMessage) != "" {
		return decision.ClientMessage
	}
	return "Request blocked by content policy"
}

func cloneSecurityAuditGroupID(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
