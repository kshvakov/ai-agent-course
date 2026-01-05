package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
)

// ToolRequest представляет запрос на выполнение инструмента
type ToolRequest struct {
	Tool      string          `json:"tool"`
	Version   string          `json:"version"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResponse представляет ответ от tool server
type ToolResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

// ToolDefinition представляет определение инструмента
type ToolDefinition struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	CompatibleWith []string        `json:"compatible_with"`
	Description    string          `json:"description"`
	Parameters     json.RawMessage `json:"parameters"`
}

// TODO 1: Реализуйте stdio protocol для tool server
// Читайте запросы из stdin, выполняйте инструменты, пишите ответы в stdout
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
	// TODO: Читайте запросы из stdin (JSON)
	// TODO: Проверяйте версию
	// TODO: Выполняйте инструмент
	// TODO: Пишите ответ в stdout (JSON)
	
	return fmt.Errorf("not implemented")
}

// TODO 2: Реализуйте HTTP protocol для tool server
// HTTP endpoint для выполнения инструментов
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
	// TODO: Создайте HTTP endpoint POST /execute
	// TODO: Обработайте запрос
	// TODO: Верните JSON ответ
	
	return fmt.Errorf("not implemented")
}

// TODO 3: Реализуйте проверку совместимости версий
func checkVersionCompatibility(tool *ToolDefinition, requestedVersion string) bool {
	// TODO: Проверьте, совместима ли запрошенная версия
	// TODO: Учтите поле CompatibleWith
	// TODO: Верните true если совместима
	
	return false
}

// TODO 4: Реализуйте tool client для агента
// Клиент для вызова tool server через протокол
type ToolClient interface {
	CallTool(tool string, version string, arguments json.RawMessage) (string, error)
}

type StdioToolClient struct {
	cmd *exec.Cmd
}

func NewStdioToolClient(serverPath string) (*StdioToolClient, error) {
	// TODO: Запустите tool server как отдельный процесс
	// TODO: Настройте stdin/stdout для коммуникации
	
	return nil, fmt.Errorf("not implemented")
}

func (c *StdioToolClient) CallTool(tool string, version string, arguments json.RawMessage) (string, error) {
	// TODO: Отправьте запрос в tool server через stdin
	// TODO: Прочитайте ответ из stdout
	// TODO: Верните результат
	
	return "", fmt.Errorf("not implemented")
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
	// TODO: Отправьте HTTP POST запрос
	// TODO: Обработайте ответ
	// TODO: Верните результат
	
	return "", fmt.Errorf("not implemented")
}

// Mock инструменты для тестирования
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
	// Пример использования stdio protocol
	fmt.Println("=== Lab 12: Tool Server Protocol ===")
	fmt.Println("Starting stdio tool server...\n")

	server := NewStdioToolServer()
	
	// Регистрируем инструмент
	server.RegisterTool(&ToolDefinition{
		Name:           "check_status",
		Version:        "1.0",
		CompatibleWith: []string{"1.0", "1.1"},
		Description:    "Check server status",
		Parameters:     json.RawMessage(`{"type": "object", "properties": {"hostname": {"type": "string"}}}`),
	})

	// TODO: Запустите сервер
	// server.Start()

	_ = bufio.NewReader(os.Stdin)
	_ = context.Background()
}

