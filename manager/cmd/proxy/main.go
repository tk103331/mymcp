package main

import (
	"flag"
	"mcphosting/manager/data"
	"mcphosting/manager/proxy"
	"net/url"
	"os"
)

func main() {
	serveFlag := flag.String("serve", "", "serve on mode, sse/stdio")
	proxyFlag := flag.String("proxy", "", "proxy to mode, sse/stdio")
	baseUrlFlag := flag.String("baseUrl", "", "the url serve on")
	urlFlag := flag.String("url", "", "sse url to proxy to")
	cmdFlag := flag.String("cmd", "", "stdio command to proxy to")
	flag.Parse()

	if *serveFlag == "" || *proxyFlag == "" {
		panic("serve and proxy flag is empty!")
	} else if *serveFlag != "sse" && *serveFlag != "stdio" {
		panic("serve flag must be one of stdio/sse !")
	} else if *proxyFlag != "sse" && *proxyFlag != "stdio" {
		panic("proxy flag must be one of stdio/sse !")
	}

	cfg := &data.ServerConfig{
		ID:        "proxy",
		Workspace: "proxy",
		Name:      "proxy",
		Type:      "proxy",
	}

	if *proxyFlag == "sse" {
		_, err := url.Parse(*urlFlag)
		if err != nil {
			panic("url flag invalid: " + err.Error())
		}
		cfg.Transport = "sse"
		cfg.Url = *urlFlag
	} else if *proxyFlag == "stdio" {
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
	if *serveFlag == "sse" {
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
