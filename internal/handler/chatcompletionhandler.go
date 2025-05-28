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

		if req.Stream {
			// Handle streaming response with SSE
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

			// Flush headers immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			err := l.ChatCompletionStream(&req, w, r.Header)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
			}
		} else {
			// Handle non-streaming response
			resp, err := l.ChatCompletion(&req, r.Header)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
			} else {
				w.Header().Set("Content-Type", "application/json")
				httpx.OkJsonCtx(r.Context(), w, resp)
			}
		}
	}
}
