package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	cmd := flag.String("cmd", "", "stdio mcp server command")
	url := flag.String("url", "", "sse mcp server url")
	flag.Parse()
	if *cmd == "" && *url == "" {
		flag.Usage()
		os.Exit(1)
	}
	c, err := initClient(*cmd, *url)
	if err != nil {
		log.Fatal(err)
	}
	// Create context with timeout
	ctx := context.Background()

	// Initialize the client
	fmt.Println("Initializing client...")
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "mcpdebug",
		Version: "1.0.0",
	}

	initResult, err := c.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	fmt.Printf(
		"Initialize MCP Server: %s %s\n\n",
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version,
	)
	// List Tools

	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	toolMap := make(map[string]mcp.Tool)

	for _, tool := range tools.Tools {
		toolMap[tool.Name] = tool
	}
	fmt.Println()
	count := len(tools.Tools)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Available tools:")
	for {
		for i, tool := range tools.Tools {
			fmt.Printf("[%d] - %s: %s\n", i+1, tool.Name, tool.Description)
		}
		fmt.Print("Input tool index or name: ")
		if !scanner.Scan() {
			continue
		}
		text := scanner.Text()
		tool, ok := toolMap[text]
		if !ok {
			index, err := strconv.Atoi(text)
			if err != nil || index < 1 || index > count {
				log.Fatalf("invalid tool index or name: %v", text)
			}
			tool = tools.Tools[index-1]
		}
		fmt.Println("Name: " + tool.Name)
		fmt.Println("Description: " + tool.Description)
		fmt.Println("InputSchema: " + toJson(tool.InputSchema))
		fmt.Println()

		waitArgsAndCall(c, scanner, tool)
		fmt.Println()
	}
}

func initClient(cmd, url string) (client.MCPClient, error) {
	if cmd != "" {
		parts := strings.Split(cmd, " ")
		env := os.Environ()
		stdioClient, err := client.NewStdioMCPClient(parts[0], env, parts[1:]...)
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}
		return stdioClient, nil
	} else if url != "" {
		sseClient, err := client.NewSSEMCPClient(url)
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}
		err = sseClient.Start(context.Background())
		if err != nil {
			return nil, err
		}
		return sseClient, nil
	} else {
		flag.Usage()
		os.Exit(1)
	}
	return nil, errors.New("invalid arguments")
}

func waitArgsAndCall(c client.MCPClient, scanner *bufio.Scanner, tool mcp.Tool) {
	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
	for {
		fmt.Println("Input arguments:")
		if !scanner.Scan() {
			continue
		}
		text := scanner.Text()
		params := make(map[string]interface{})
		err := json.Unmarshal([]byte(text), &params)
		if err != nil {
			fmt.Println("Invalid arguments:" + err.Error())
			continue
		}

		request := mcp.CallToolRequest{}
		request.Params.Name = tool.Name
		request.Params.Arguments = params

		result, err := c.CallTool(ctx, request)
		if err != nil {
			log.Fatalf("Failed to call tool: %v", err)
		}
		printToolResult(result)
		fmt.Println()
	}
}

func printToolResult(result *mcp.CallToolResult) {
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		} else {
			jsonBytes, _ := json.MarshalIndent(content, "", "  ")
			fmt.Println(string(jsonBytes))
		}
	}
}

func toJson(obj any) string {
	data, _ := json.MarshalIndent(obj, "", " ")
	return string(data)
}
