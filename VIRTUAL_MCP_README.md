# Virtual MCP Server

The Virtual MCP Server is a dynamic MCP (Model Context Protocol) server that exposes Obot project tasks as MCP tools. It allows external MCP clients to interact with Obot's internal task system through the standard MCP protocol.

## Overview

The Virtual MCP Server:

- Dynamically creates MCP servers per project
- Loads project tasks and exposes them as MCP tools
- Registers tools only when an `initialize` message is received
- Uses project ID from the URL path instead of headers
- Generates input schemas from task parameters

## Authentication

The server uses a secret-based authentication system:

- A secret is generated on startup and stored in memory
- The secret can be retrieved via the `/api/virtual-mcp/secret` endpoint
- All requests must include the `x-virtual-mcp-secret` header with the correct secret
- Invalid or missing secrets result in 401 Unauthorized responses

## Usage

### Getting the Secret

```bash
curl http://localhost:8080/api/virtual-mcp/secret
```

Response:

```json
{
  "secret": "a1b2c3d4e5f6..."
}
```

### Connecting to a Project

Connect to a specific project using the project ID in the URL path:

```
/virtual/mcp/{project_id}
```

Example:

```
/virtual/mcp/project-abc123
```

### MCP Protocol Flow

1. **Connect**: Connect to `/virtual/mcp/{project_id}`
2. **Initialize**: Send an `initialize` message (tools are registered at this point)
3. **List Tools**: Use `tools/list` to see available tools
4. **Call Tools**: Use `tools/call` to execute tasks

### Example MCP Messages

#### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-06-18",
    "capabilities": {
      "roots": {
        "listChanged": true
      },
      "sampling": {},
      "elicitation": {}
    },
    "clientInfo": {
      "name": "Visual Studio Code",
      "version": "1.102.0"
    }
  }
}
```

**Headers required:**

```
Content-Type: application/json
x-virtual-mcp-secret: your-secret-here
```

#### List Tools

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

**Headers required:**

```
Content-Type: application/json
x-virtual-mcp-secret: your-secret-here
```

#### Call Tool

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "task-name",
    "arguments": {
      "input": "test input",
      "format": "json"
    }
  }
}
```

**Headers required:**

```
Content-Type: application/json
x-virtual-mcp-secret: your-secret-here
```

## Implementation Details

### Architecture

- **Project-based Servers**: Each project gets its own MCP server instance
- **Lazy Tool Registration**: Tools are only registered when an `initialize` message is received
- **Message Interception**: HTTP requests are intercepted to detect `initialize` messages
- **Thread-safe**: Uses read-write mutexes for concurrent access to project servers

### Key Components

1. **VirtualMCPHandler**: Main handler that manages project servers
2. **ProjectServer**: Contains server, handler, and tasks for a specific project
3. **Message Interception**: Detects `initialize` messages and triggers tool registration
4. **Task Execution**: Executes Obot tasks and returns results as MCP tool responses

### Tool Generation

For each task in a project:

- **Name**: Uses `task.Spec.Manifest.Name`
- **Description**: Uses `task.Spec.Manifest.Description`
- **Input Schema**: Generated from `task.Spec.Manifest.Params`
- **Execution**: Calls the task via `invoke.Invoker.Workflow()`

### Input Schema Generation

Task parameters are converted to JSON schema:

```go
// Example task parameters
params:
  input: "default input"
  format: "json"

// Generated schema
{
  "type": "object",
  "properties": {
    "input": {
      "type": "string",
      "description": "Parameter: input",
      "default": "default input"
    },
    "format": {
      "type": "string",
      "description": "Parameter: format",
      "default": "json"
    }
  },
  "required": ["input", "format"]
}
```

## Example Go Client

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "encoding/json"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
    // Get the secret first
    resp, err := http.Get("http://localhost:8080/api/virtual-mcp/secret")
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()

    var secretResp map[string]string
    if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
        log.Fatal(err)
    }
    secret := secretResp["secret"]

    // Create client with custom headers
    client := mcp.NewClient("virtual-mcp-client")

    // Create custom transport with headers
    transport := &http.Transport{}
    client.SetTransport(transport)

    // Connect to project with secret header
    conn, err := client.ConnectWithHeaders(context.Background(),
        "http://localhost:8080/virtual/mcp/project-abc123",
        map[string]string{
            "x-virtual-mcp-secret": secret,
        })
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Initialize (this triggers tool registration)
    err = client.Initialize(context.Background(), conn, &mcp.InitializeParams{
        ProtocolVersion: "2025-06-18",
        Capabilities: &mcp.ClientCapabilities{},
        ClientInfo: &mcp.ClientInfo{
            Name:    "test-client",
            Version: "1.0.0",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // List tools
    tools, err := client.ListTools(context.Background(), conn)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Available tools: %d\n", len(tools.Tools))
    for _, tool := range tools.Tools {
        fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
    }

    // Call a tool
    result, err := client.CallTool(context.Background(), conn, &mcp.CallToolParams{
        Name: "my-task",
        Arguments: map[string]interface{}{
            "input":  "test input",
            "format": "json",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Tool result: %s\n", result.Content[0].(*mcp.TextContent).Text)
}
```

## Error Handling

- **Project Not Found**: Returns 404 if project ID is invalid
- **Task Execution Errors**: Returns error details in tool call responses
- **Invalid Requests**: Returns appropriate HTTP status codes
- **Tool Registration Errors**: Logs errors but continues operation

## Limitations

- **No Tool Removal**: Once tools are registered, they cannot be removed
- **Project-specific**: Each project requires a separate connection
- **Memory Usage**: Project servers are kept in memory
- **No Persistence**: Project servers are lost on restart

## Security Considerations

- **Secret-based Auth**: All requests require the `x-virtual-mcp-secret` header
- **Project Isolation**: Each project has its own server instance
- **Input Validation**: Task parameters are validated by the task system
- **Output Sanitization**: Task outputs are returned as-is
- **Secret Generation**: Secret is generated once on startup and remains constant

## Future Enhancements

- **Authentication**: Re-enable secret-based authentication
- **Tool Updates**: Support for dynamic tool updates
- **Connection Pooling**: Reuse connections for better performance
- **Metrics**: Add monitoring and metrics collection
- **Caching**: Cache project data for better performance
