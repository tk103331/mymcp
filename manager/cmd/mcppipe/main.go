package main

import (
	"flag"
	"mcphosting/manager/data"
	"mcphosting/manager/proxy"
	"net/url"
	"os"
	"strings"
)

func main() {
	modeFlag := flag.String("mode", "", "proxy mode, stdio2sse/sse2stdio/sse2sse/stdio2stdio")
	baseUrlFlag := flag.String("baseUrl", "", "the url serve on")
	urlFlag := flag.String("url", "", "sse url to proxy to")
	cmdFlag := flag.String("cmd", "", "stdio command to proxy to")
	flag.Parse()

	if *modeFlag != "stdio2sse" && *modeFlag != "sse2stdio" && *modeFlag != "sse2sse" && *modeFlag != "stdio2stdio" {
		panic("proxy mode must be one of stdio2sse/sse2stdio/sse2sse/stdio2stdio")
	}

	parts := strings.Split(*modeFlag, "2")
	proxyFlag := parts[0]
	serveFlag := parts[1]

	cfg := &data.ServerConfig{
		ID:        "proxy",
		Workspace: "proxy",
		Name:      "proxy",
		Type:      "proxy",
		Cmd:       *cmdFlag,
		Url:       *urlFlag,
	}

	if proxyFlag == "sse" {
		_, err := url.Parse(*urlFlag)
		if err != nil {
			panic("url flag invalid: " + err.Error())
		}
		cfg.Transport = "sse"
		cfg.Url = *urlFlag
	} else if proxyFlag == "stdio" {
		if *cmdFlag == "" {
			panic("stdio command is empty!")
		}
		cfg.Cmd = *cmdFlag
		cfg.Env = os.Environ()
	}

	_, err := url.Parse(*urlFlag)
	if err != nil {
		panic(err)
	}

	proxyServer, err := proxy.NewProxyServer(cfg)

	if err != nil {
		panic(err)
	}
	if serveFlag == "sse" {
		parsedBaseUrl, err := url.Parse(*baseUrlFlag)
		if err != nil {
			panic("baseUrl flag invalid: " + err.Error())
		}
		sseServer := proxy.NewSSEServer(proxyServer, proxy.WithBaseURL(*baseUrlFlag))
		err = sseServer.Start(parsedBaseUrl.Host)
	} else {
		err = proxy.ServeStdio(proxyServer)
		if err != nil {
			panic(err)
		}
	}

}
