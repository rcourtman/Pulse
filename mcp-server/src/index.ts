#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import { exec } from "child_process";
import { promisify } from "util";
import { writeFile, readFile } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { randomBytes } from "crypto";

const execAsync = promisify(exec);

// Session management
const sessions = new Map<string, string>(); // Map conversation ID to codex session ID

// Create the MCP server
const server = new McpServer({
  name: "pulse-codex",
  version: "1.0.0",
});

// Register the codex tool for initial questions
server.registerTool(
  "codex",
  {
    title: "Codex AI Assistant",
    description: "Ask a question to OpenAI Codex AI assistant. Returns a thoughtful response. Use this for architectural decisions, code review, and technical consultation.",
    inputSchema: {
      question: z.string().describe("The question to ask Codex"),
      conversationId: z.string().optional().describe("Optional conversation ID to maintain context across calls"),
    },
  },
  async ({ question, conversationId }) => {
    const convId = conversationId || `conv-${Date.now()}-${process.pid}`;
    const uniqueId = `${Date.now()}-${process.pid}-${randomBytes(4).toString('hex')}`;
    const tempFile = join(tmpdir(), `codex-${uniqueId}.txt`);

    try {
      // Check if we have an existing codex session for this conversation
      const existingSessionId = sessions.get(convId);

      // Write question to a temp file to avoid shell escaping issues
      const questionFile = join(tmpdir(), `codex-q-${uniqueId}.txt`);
      await writeFile(questionFile, question, "utf-8");

      let command: string;
      if (existingSessionId) {
        // Resume existing session
        command = `codex exec --yolo -o "${tempFile}" resume "${existingSessionId}" < "${questionFile}"`;
      } else {
        // Start new session
        command = `codex exec --yolo -o "${tempFile}" < "${questionFile}"`;
      }

      // Execute codex and capture stderr for session ID only
      // Note: stderr contains all reasoning output, but we only need the session ID
      const { stdout, stderr } = await execAsync(command, {
        timeout: 1800000, // 30 minute timeout (codex can take a very long time for complex queries)
        maxBuffer: 10 * 1024 * 1024, // 10MB buffer
      });

      // Extract only the session ID from stderr
      const sessionIdMatch = stderr.match(/session id:\s*([a-f0-9-]+)/);
      const sessionId = sessionIdMatch ? sessionIdMatch[1] : "";

      // Store session for this conversation
      if (sessionId && !existingSessionId) {
        sessions.set(convId, sessionId);
      }

      // Read the response from the output file
      const response = await readFile(tempFile, "utf-8");

      return {
        content: [
          {
            type: "text",
            text: `${response.trim()}\n\n---\nSession ID: ${sessionId}\nConversation ID: ${convId}`,
          },
        ],
      };
    } catch (error) {
      // Log full error to stderr for debugging
      console.error("Codex execution error:", error);

      // Extract only the relevant error info, not the full stderr with reasoning
      let errorMessage = "Unknown error";
      if (error instanceof Error) {
        // For exec errors, the message contains both stdout and stderr
        // We want to extract just the actual error, not all the reasoning output
        const lines = error.message.split('\n');
        // Look for actual error messages (usually start with "Error:" or contain "failed")
        const relevantLines = lines.filter(line =>
          line.includes('Error:') ||
          line.includes('failed') ||
          line.includes('not found') ||
          line.includes('permission denied') ||
          line.includes('exit code')
        );
        errorMessage = relevantLines.length > 0 ? relevantLines.join('\n') : error.message;
      }
      throw new Error(`Failed to execute codex: ${errorMessage}`);
    }
  }
);

// Register a tool to clear session history
server.registerTool(
  "codex_clear_session",
  {
    title: "Clear Codex Session",
    description: "Clear the stored codex session for a conversation, starting fresh",
    inputSchema: {
      conversationId: z.string().describe("The conversation ID to clear"),
    },
  },
  async ({ conversationId }) => {
    const existed = sessions.has(conversationId);
    sessions.delete(conversationId);

    const message = existed
      ? `Session cleared for conversation ${conversationId}`
      : `No session found for conversation ${conversationId}`;

    return {
      content: [
        {
          type: "text",
          text: message,
        },
      ],
    };
  }
);

// Connect to stdio transport
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);

  // Log to stderr so it doesn't interfere with MCP protocol
  console.error("Pulse Codex MCP Server running on stdio");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
