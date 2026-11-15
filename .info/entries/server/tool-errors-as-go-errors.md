---
description: Why tool failures are returned as a Go error carrying JSON text, not as structuredContent.
tags: [errors, mcp-sdk]
---
Tools are registered with typed outputs (`mcp.AddTool` with an `Out` type), so go-sdk validates any `structuredContent` against the output schema. An error object doesn't match that schema, so it can't go in `structuredContent`. Instead the handler in `internal/server/server.go` returns a Go error whose `Error()` is the problem JSON; the SDK then sets `isError: true` and puts that text in the content.

Clients therefore parse the error JSON from the text content. Non-`Problem` errors are wrapped as `io_error` so every failure has the same shape. Malformed arguments never reach the handler: the SDK's input schema rejects them with its own plain-text message.
