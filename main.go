package main

import (
	"flag"

	"github.com/zgsm-ai/chat-rag/internal/config"
	"github.com/zgsm-ai/chat-rag/internal/handler"
	"github.com/zgsm-ai/chat-rag/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

// main is the entry point of the chat-rag service
func main() {
	var configFile string
	flag.StringVar(&configFile, "f", "etc/chat-api.yaml", "the config file")
	flag.Parse()

	var c config.Config
	conf.MustLoad(configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
