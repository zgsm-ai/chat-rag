package handler

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zgsm-ai/chat-rag/internal/logic"
	"github.com/zgsm-ai/chat-rag/internal/svc"
	"github.com/zgsm-ai/chat-rag/internal/types"
)

// ChatCompletionHandler handles chat completion requests
func ChatCompletionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatCompletionRequest
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewChatCompletionLogic(r.Context(), svcCtx)
		resp, err := l.ChatCompletion(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			// Set appropriate headers for OpenAI compatibility
			w.Header().Set("Content-Type", "application/json")
			if req.Stream {
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.Header().Set("Transfer-Encoding", "chunked")
			}
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
