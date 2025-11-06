# Pulse Codex MCP Server

An MCP (Model Context Protocol) server that provides OpenAI Codex integration for Claude Code.

## Features

- **codex** - Ask questions to OpenAI Codex AI assistant
- **codex_clear_session** - Clear stored session context
- Automatic session management across conversation turns
- Clean response extraction (no token counts or debug info)

## Installation

### 1. Build the Server

```bash
cd /opt/pulse/mcp-server
npm install
npm run build
```

### 2. Add to Claude Code

```bash
claude mcp add --transport stdio pulse-codex -- node /opt/pulse/mcp-server/dist/index.js
```

## Usage

### In Claude Code Conversations

**Ask a question:**
```
Use the codex tool to evaluate whether we should use X or Y for Z scenario
```

**Continue a conversation:**
```
Use the codex tool with conversationId "my-conversation" to dig deeper into that approach
```

**Start fresh:**
```
Use the codex_clear_session tool with conversationId "my-conversation"
```

## How It Works

- Each conversation is identified by a `conversationId`
- The server automatically maintains codex session IDs for each conversation
- Responses are clean (just the answer, no metadata)
- Sessions persist across multiple tool calls with the same conversationId

## Development

Watch mode for development:
```bash
npm run dev
```

## Architecture

- **TypeScript** - Type-safe MCP server implementation
- **@modelcontextprotocol/sdk** - Official MCP SDK
- **stdio transport** - Communicates with Claude Code via standard input/output
