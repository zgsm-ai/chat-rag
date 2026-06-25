package service

import (
	"testing"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/model"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

func newErrorLog(user string) *model.ChatLog {
	log := &model.ChatLog{
		Error: []map[types.ErrorType]string{
			{types.ErrApiError: "boom"},
		},
	}
	log.Identity.UserName = user
	return log
}

func TestShouldSaveErrorLog_AllMode(t *testing.T) {
	ls := &LoggerRecordService{errorLogMode: config.ErrorLogModeAll}
	if !ls.shouldSaveErrorLog(newErrorLog("alice")) {
		t.Fatal("expected save in all mode")
	}
}

func TestShouldSaveErrorLog_NoneMode(t *testing.T) {
	ls := &LoggerRecordService{errorLogMode: config.ErrorLogModeNone}
	if ls.shouldSaveErrorLog(newErrorLog("alice")) {
		t.Fatal("expected skip in none mode")
	}
}

func TestShouldSaveErrorLog_SampledMode(t *testing.T) {
	ls := &LoggerRecordService{
		errorLogMode: config.ErrorLogModeSampled,
		errorSampler: NewErrorLogSampler(1, 60),
	}
	if !ls.shouldSaveErrorLog(newErrorLog("alice")) {
		t.Fatal("expected first sampled save for alice to be allowed")
	}
	if ls.shouldSaveErrorLog(newErrorLog("alice")) {
		t.Fatal("expected second sampled save for alice to be dropped")
	}
	if !ls.shouldSaveErrorLog(newErrorLog("bob")) {
		t.Fatal("expected sampled save for bob (separate user) to be allowed")
	}
}

func TestShouldSaveErrorLog_SampledModeNilSampler(t *testing.T) {
	ls := &LoggerRecordService{errorLogMode: config.ErrorLogModeSampled}
	if !ls.shouldSaveErrorLog(newErrorLog("alice")) {
		t.Fatal("expected save when sampler is nil (fail-open)")
	}
}

func TestFirstErrorTypeKey(t *testing.T) {
	if got := firstErrorTypeKey(newErrorLog("alice")); got != string(types.ErrApiError) {
		t.Errorf("firstErrorTypeKey() = %q, want %q", got, types.ErrApiError)
	}
	if got := firstErrorTypeKey(&model.ChatLog{}); got != "" {
		t.Errorf("firstErrorTypeKey() on empty = %q, want empty", got)
	}
	emptyMap := &model.ChatLog{Error: []map[types.ErrorType]string{{}}}
	if got := firstErrorTypeKey(emptyMap); got != "" {
		t.Errorf("firstErrorTypeKey() on empty map = %q, want empty", got)
	}
}
