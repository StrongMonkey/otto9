package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/invoke"
	v1 "github.com/obot-platform/obot/pkg/storage/apis/obot.obot.ai/v1"
	"github.com/obot-platform/obot/pkg/system"
	"github.com/obot-platform/obot/pkg/virtualsecret"
	"github.com/obot-platform/obot/pkg/wait"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// VirtualMCPServer represents a separate HTTP server for virtual MCP functionality
//
// Usage example:
//
//	virtualServer, err := NewVirtualMCPServer(storageClient, invoker, authenticator)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get the port and URL for external use
//	port := virtualServer.GetPort()
//	url := virtualServer.GetURL()
//	fmt.Printf("Virtual MCP server running on port %d: %s\n", port, url)
//
//	// Start the server in a goroutine
//	go func() {
//	    if err := virtualServer.Start(); err != nil && err != http.ErrServerClosed {
//	        log.Printf("Virtual MCP server error: %v", err)
//	    }
//	}()
//
//	// Later, to shutdown:
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	virtualServer.Shutdown(ctx)
//
// Integration with main server:
//
//	In your main server setup (e.g., pkg/server/server.go), you can add:
//
//	// Create and start virtual MCP server
//	virtualMCPServer, err := handlers.NewVirtualMCPServer(storageClient, invoker, authenticator)
//	if err != nil {
//	    log.Fatalf("Failed to create virtual MCP server: %v", err)
//	}
//
//	// Log the virtual MCP server details
//	log.Infof("Virtual MCP server started on port %d", virtualMCPServer.GetPort())
//	log.Infof("Virtual MCP secret endpoint: %s", virtualMCPServer.GetSecretURL())
//
//	// Start virtual MCP server in background
//	go func() {
//	    if err := virtualMCPServer.Start(); err != nil && err != http.ErrServerClosed {
//	        log.Errorf("Virtual MCP server error: %v", err)
//	    }
//	}()
//
//	// Add shutdown handling
//	context.AfterFunc(ctx, func() {
//	    log.Infof("Shutting down virtual MCP server")
//	    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	    defer cancel()
//	    if err := virtualMCPServer.Shutdown(shutdownCtx); err != nil {
//	        log.Errorf("Failed to shutdown virtual MCP server: %v", err)
//	    }
//	})
type VirtualMCPServer struct {
	handler *VirtualMCPHandler
	server  *http.Server
	port    int
}

// NewVirtualMCPServer creates a new virtual MCP server that listens on a random port
func NewVirtualMCPServer(c kclient.WithWatch, invoker *invoke.Invoker) (*VirtualMCPServer, error) {
	handler := NewVirtualMCPHandler(c, invoker)

	mux := http.NewServeMux()

	mux.HandleFunc("/virtual/mcp/{project_id}", handler.StreamableHTTP)

	// Get a random available port
	listener, err := net.Listen("tcp", ":8090")
	if err != nil {
		return nil, fmt.Errorf("failed to get random port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	return &VirtualMCPServer{
		handler: handler,
		server:  server,
		port:    8090,
	}, nil
}

// GetPort returns the port number the virtual MCP server is listening on
func (s *VirtualMCPServer) GetPort() int {
	return s.port
}

// GetURL returns the base URL for the virtual MCP server
func (s *VirtualMCPServer) GetURL() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

// GetSecretURL returns the URL for the secret endpoint
func (s *VirtualMCPServer) GetSecretURL() string {
	return fmt.Sprintf("http://localhost:%d/api/virtual-mcp/secret", s.port)
}

// Start starts the virtual MCP server
func (s *VirtualMCPServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the virtual MCP server
func (s *VirtualMCPServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// ProjectServer represents a server and handler pair for a specific project
type ProjectServer struct {
	Server          *mcp.Server
	Handler         *mcp.StreamableHTTPHandler
	Tasks           []v1.Workflow
	ToolsRegistered bool
}

type VirtualMCPHandler struct {
	c            kclient.WithWatch
	invoker      *invoke.Invoker
	projectMap   map[string]*ProjectServer
	projectMutex sync.RWMutex
}

func NewVirtualMCPHandler(c kclient.WithWatch, invoker *invoke.Invoker) *VirtualMCPHandler {
	return &VirtualMCPHandler{
		invoker:    invoker,
		projectMap: make(map[string]*ProjectServer),
		c:          c,
	}
}

// StreamableHTTP handles the virtual MCP HTTP endpoint
func (h *VirtualMCPHandler) StreamableHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate the secret header
	secret := r.Header.Get("x-virtual-mcp-secret")
	if secret == "" || secret != virtualsecret.GetVirtualMCPSecret() {
		http.Error(w, "Invalid or missing x-virtual-mcp-secret header", http.StatusUnauthorized)
		return
	}

	// Get project ID from path parameter
	fmt.Println("debugggg")
	projectID := strings.TrimPrefix(r.URL.Path, "/virtual/mcp/")
	if projectID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing project_id in path"))
		return
	}

	// Get or create project server
	projectServer := h.getOrCreateProjectServer(r, projectID)
	if projectServer == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to create project server"))
		return
	}
	fmt.Println("debugggg444444")

	// Use custom handler that intercepts messages
	h.serveWithMessageInterception(w, r, projectServer, projectID)
	fmt.Println("1123123123")
}

// serveWithMessageInterception intercepts MCP messages to detect initialize calls
func (h *VirtualMCPHandler) serveWithMessageInterception(w http.ResponseWriter, r *http.Request, projectServer *ProjectServer, projectID string) {
	// Check if this is a POST request with JSON content
	if r.Method == "POST" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		// Try to parse as JSON-RPC message
		var message map[string]interface{}
		if err := json.Unmarshal(body, &message); err == nil {
			// Check if this is an initialize message
			if method, ok := message["method"].(string); ok && method == "initialize" {
				// Register tools for this project
				if !projectServer.ToolsRegistered {
					if err := h.registerToolsForProject(projectID); err == nil {
						projectServer.ToolsRegistered = true
					}
				}
			}
		}

		// Create a new request with the body
		r.Body = io.NopCloser(strings.NewReader(string(body)))
	}

	// Serve the request normally
	projectServer.Handler.ServeHTTP(w, r)
}

// getOrCreateProjectServer retrieves or creates a server for a specific project
func (h *VirtualMCPHandler) getOrCreateProjectServer(req *http.Request, projectID string) *ProjectServer {
	h.projectMutex.RLock()
	if server, exists := h.projectMap[projectID]; exists {
		h.projectMutex.RUnlock()
		return server
	}
	h.projectMutex.RUnlock()

	// Need to create a new server
	h.projectMutex.Lock()
	defer h.projectMutex.Unlock()

	// Double-check after acquiring write lock
	if server, exists := h.projectMap[projectID]; exists {
		return server
	}

	// Convert project ID to thread ID
	threadID := strings.Replace(projectID, system.ProjectPrefix, system.ThreadPrefix, 1)

	// Load the project thread
	var thread v1.Thread
	if err := h.c.Get(req.Context(), kclient.ObjectKey{
		Namespace: "default",
		Name:      threadID,
	}, &thread); err != nil {
		return nil
	}

	// Load tasks for this project
	tasks, err := h.loadProjectTasks(req, &thread)
	if err != nil {
		return nil
	}

	// Create new server and handler
	server := mcp.NewServer(&mcp.Implementation{Name: "virtual-mcp"}, nil)

	// Create a handler
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	projectServer := &ProjectServer{
		Server:          server,
		Handler:         handler,
		Tasks:           tasks,
		ToolsRegistered: false,
	}

	// Store the project server
	h.projectMap[projectID] = projectServer

	return projectServer
}

// loadProjectTasks loads all tasks for a given project thread
func (h *VirtualMCPHandler) loadProjectTasks(req *http.Request, thread *v1.Thread) ([]v1.Workflow, error) {
	var workflows v1.WorkflowList
	if err := h.c.List(req.Context(), &workflows, kclient.MatchingFields{
		"spec.threadName": thread.Name,
	}); err != nil {
		return nil, err
	}

	return workflows.Items, nil
}

// registerToolsForProject registers all tools for a specific project
func (h *VirtualMCPHandler) registerToolsForProject(projectID string) error {
	h.projectMutex.RLock()
	projectServer, exists := h.projectMap[projectID]
	h.projectMutex.RUnlock()

	if !exists {
		return fmt.Errorf("project server not found for project ID: %s", projectID)
	}

	// Add tools for each task
	for _, task := range projectServer.Tasks {
		taskCopy := task // Create a copy for the closure

		// Create input schema from task parameters
		var inputSchema *jsonschema.Schema
		if len(task.Spec.Manifest.Params) > 0 {
			properties := make(map[string]*jsonschema.Schema)
			required := make([]string, 0)

			for key, value := range task.Spec.Manifest.Params {
				properties[key] = &jsonschema.Schema{
					Type:        "string",
					Description: value,
				}
				required = append(required, key)
			}

			schema := jsonschema.Schema{
				Type:       "object",
				Properties: properties,
			}
			if len(required) > 0 {
				schema.Required = required
			}
			inputSchema = &schema
		}

		// Normalize task name by removing spaces
		normalizedName := strings.ReplaceAll(task.Spec.Manifest.Name, " ", "")

		mcp.AddTool(projectServer.Server, &mcp.Tool{
			Name:        normalizedName,
			Description: task.Spec.Manifest.Description,
			InputSchema: inputSchema,
		}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]interface{}]) (*mcp.CallToolResultFor[any], error) {
			return h.executeTask(ctx, &taskCopy, params.Arguments)
		})
	}

	return nil
}

// executeTask executes a task and returns the result
func (h *VirtualMCPHandler) executeTask(ctx context.Context, task *v1.Workflow, args map[string]interface{}) (*mcp.CallToolResultFor[any], error) {
	// Convert arguments to JSON input
	input, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task input: %w", err)
	}

	// Execute the task
	resp, err := h.invoker.Workflow(ctx, h.c, task, string(input), invoke.WorkflowOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute task: %w", err)
	}
	defer resp.Close()

	// Wait for the workflow execution to have status output
	wfe, err := wait.For(ctx, h.c, resp.WorkflowExecution, func(wfe *v1.WorkflowExecution) (bool, error) {
		if wfe.Status.State == types.WorkflowStateError {
			return false, fmt.Errorf("workflow failed: %s", wfe.Status.Error)
		}
		return wfe.Status.State.IsTerminal() && wfe.Status.Output != "", nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for workflow execution: %w", err)
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: wfe.Status.Output,
			},
		},
	}, nil
}
