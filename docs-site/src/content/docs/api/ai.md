---
title: AI API
description: REST + SSE surface for conversations and the streaming Claude bridge.
---

All endpoints require `Authorization: Bearer <token>`.

## Conversations

| Method + path | Notes |
|---|---|
| `GET /api/ai/conversations` | List non-archived, ordered newest first, max 100 |
| `POST /api/ai/conversations` | Create; optional `{title}` body |
| `DELETE /api/ai/conversations/:id` | Soft delete (archived = 1) |
| `GET /api/ai/conversations/:id/messages` | Full message history |

Conversation DTO:

```json
{
  "id": "...",
  "title": "Should I push hard today?",
  "started_at": 1747000000,
  "pillar_scope": []
}
```

Message DTO:

```json
{
  "id": "...",
  "role": "user",
  "content": [{ "type": "text", "text": "Should I push hard today?" }],
  "tool_calls": null,
  "created_at": 1747000000
}
```

Content blocks are Anthropic-shaped — `text`, `tool_use`, `tool_result`. The
iOS app flattens these to text + a list of tool names for display.

## Streaming a turn

`POST /api/ai/conversations/:id/messages`

```json
{ "text": "Should I push hard today?" }
```

Response is `Content-Type: text/event-stream`. Each event is one of:

| Type | Fields | Meaning |
|---|---|---|
| `text` | `text` | A chunk of streamed assistant text |
| `tool_call` | `tool_name` | Server is about to execute a tool |
| `tool_error` | `tool_name`, `error` | Tool execution failed — assistant continues with the error in its context, UI can show inline |
| `done` | — | Stream ended successfully |
| `error` | `error` | Fatal — abort |

Example event:

```
data: {"type":"tool_call","tool_name":"health_today"}

```

The server persists both the user message and the full assistant content
blocks (including tool uses + results) to `ai_messages` after the stream
completes. The conversation is auto-titled from the first user message
(trimmed to 60 chars) if it's still untitled.

## Tool execution

Each `tool_call` event corresponds to a Claude-issued tool use. The server
runs the tool against SQLite, returns the result back to Claude as a
`tool_result` content block, and continues the streaming loop. Maximum 8
turns per request.

See [Ask Claude](/pillars/ask/) for the full tool list.
