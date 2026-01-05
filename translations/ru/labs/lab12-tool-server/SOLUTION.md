# Решение: Lab 12 — Tool Server Protocol

## 📝 Разбор решения

### Ключевые моменты

1. **stdio Protocol:** Читайте из stdin, пишите в stdout, формат JSON.

2. **HTTP Protocol:** REST endpoint для выполнения инструментов.

3. **Версионирование:** Проверяйте совместимость версий перед выполнением.

4. **Tool Client:** Клиент для вызова tool server из агента.

### 🔍 Полное решение

```go
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
)

type ToolRequest struct {
	Tool      string          `json:"tool"`
	Version   string          `json:"version"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

type ToolDefinition struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	CompatibleWith []string        `json:"compatible_with"`
	Description    string          `json:"description"`
	Parameters     json.RawMessage `json:"parameters"`
}

type StdioToolServer struct {
	tools map[string]*ToolDefinition
}

func NewStdioToolServer() *StdioToolServer {
	return &StdioToolServer{
		tools: make(map[string]*ToolDefinition),
	}
}

func (s *StdioToolServer) RegisterTool(tool *ToolDefinition) {
	s.tools[tool.Name] = tool
}

func (s *StdioToolServer) Start() error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var req ToolRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			encoder.Encode(ToolResponse{
				Success: false,
				Error:   "Invalid JSON",
			})
			continue
		}

		tool := s.tools[req.Tool]
		if tool == nil {
			encoder.Encode(ToolResponse{
				Success: false,
				Error:   fmt.Sprintf("Tool %s not found", req.Tool),
			})
			continue
		}

		if !checkVersionCompatibility(tool, req.Version) {
			encoder.Encode(ToolResponse{
				Success: false,
				Error:   fmt.Sprintf("Version mismatch: requested %s, tool version %s", req.Version, tool.Version),
			})
			continue
		}

		result, err := executeTool(req.Tool, req.Arguments)
		if err != nil {
			encoder.Encode(ToolResponse{
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		encoder.Encode(ToolResponse{
			Success: true,
			Result:  result,
		})
	}

	return scanner.Err()
}

type HTTPToolServer struct {
	tools map[string]*ToolDefinition
}

func NewHTTPToolServer() *HTTPToolServer {
	return &HTTPToolServer{
		tools: make(map[string]*ToolDefinition),
	}
}

func (s *HTTPToolServer) RegisterTool(tool *ToolDefinition) {
	s.tools[tool.Name] = tool
}

func (s *HTTPToolServer) Start(port string) error {
	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ToolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tool := s.tools[req.Tool]
		if tool == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ToolResponse{
				Success: false,
				Error:   fmt.Sprintf("Tool %s not found", req.Tool),
			})
			return
		}

		if !checkVersionCompatibility(tool, req.Version) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ToolResponse{
				Success: false,
				Error:   fmt.Sprintf("Version mismatch: requested %s, tool version %s", req.Version, tool.Version),
			})
			return
		}

		result, err := executeTool(req.Tool, req.Arguments)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ToolResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ToolResponse{
			Success: true,
			Result:  result,
		})
	})

	return http.ListenAndServe(":"+port, nil)
}

func checkVersionCompatibility(tool *ToolDefinition, requestedVersion string) bool {
	if tool.Version == requestedVersion {
		return true
	}

	for _, compatible := range tool.CompatibleWith {
		if compatible == requestedVersion {
			return true
		}
	}

	return false
}

type HTTPToolClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPToolClient(baseURL string) *HTTPToolClient {
	return &HTTPToolClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (c *HTTPToolClient) CallTool(tool string, version string, arguments json.RawMessage) (string, error) {
	req := ToolRequest{
		Tool:      tool,
		Version:   version,
		Arguments: arguments,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Post(c.baseURL+"/execute", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var toolResp ToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
		return "", err
	}

	if !toolResp.Success {
		return "", fmt.Errorf("tool error: %s", toolResp.Error)
	}

	return toolResp.Result, nil
}

func executeTool(toolName string, arguments json.RawMessage) (string, error) {
	switch toolName {
	case "check_status":
		return "Server is ONLINE", nil
	case "restart_service":
		return "Service restarted successfully", nil
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func main() {
	// Пример HTTP сервера
	server := NewHTTPToolServer()
	server.RegisterTool(&ToolDefinition{
		Name:           "check_status",
		Version:        "1.0",
		CompatibleWith: []string{"1.0", "1.1"},
		Description:    "Check server status",
		Parameters:     json.RawMessage(`{"type": "object"}`),
	})

	fmt.Println("Starting HTTP tool server on :8080")
	if err := server.Start("8080"); err != nil {
		panic(err)
	}
}
```

