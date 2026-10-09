// MCPClientPort — domain port for the Angular side of the MCP monolith.
//
// The frontend talks to the Go MCP server (see
// openspec/changes/mcp-monolith/specs/mcp/spec.md) through this port.
// The application layer (use cases) depends on this interface; the
// infrastructure layer provides the HTTP+SSE implementation
// (infrastructure/adapters/mcp-client.adapter.ts).
//
// Hexagonal contract: this file imports ONLY rxjs. No @angular/*,
// no fetch, no DOM types. Tests can wire a fake MCPClientPort
// without HTTP mocks.
//
// The three methods mirror the spec's MCPClientAdapter requirement:
//
//   - listTools(): discover the registered tools (OpenAI-compatible
//     schemas). GET /mcp/tools/list.
//   - callTool<T>(name, args): synchronously execute a tool.
//     POST /mcp/tools/call with JSON-RPC 2.0 envelope.
//   - streamTool<T>(name, args): long-running tools stream via SSE.
//     GET /mcp/events.
import { Observable } from 'rxjs';

/**
 * MCPTool is the wire shape `GET /mcp/tools/list` returns per item.
 *
 * Fields mirror the OpenAI function-calling contract:
 *   - name:        globally unique tool name ("mcp-<group>.<action>")
 *   - description: human-readable description, includes Args/Returns
 *   - parameters:  JSON Schema object (properties + required)
 *
 * Kept as `unknown` for `parameters` so the frontend can pass the
 * schema to the LLM verbatim without a TS-side re-shape. Use a type
 * assertion at the call site if a stricter shape is needed.
 */
export interface MCPTool {
  readonly name: string;
  readonly description: string;
  readonly parameters: unknown;
}

/**
 * MCPClientPort is the application-facing contract for invoking MCP
 * tools. Implementations MUST inject user_id from the request context
 * (the adapter reads it from the Supabase JWT and stamps the
 * Authorization header; the Go server extracts it via the existing
 * JWT middleware).
 */
export interface MCPClientPort {
  /**
   * List all registered tools with their OpenAI-compatible schemas.
   * Used by the tutor chat panel to populate the function-calling
   * prompt and by the "tools" debug view.
   */
  listTools(): Observable<MCPTool[]>;

  /**
   * Synchronously call a tool. Returns the tool result as
   * Observable<T>; T is the consumer's interpretation of the tool's
   * documented return shape (the wire format is JSON).
   *
   * The adapter wraps the call in JSON-RPC 2.0 (`jsonrpc`, `id`,
   * `method: tools/call`, `params: {name, arguments}`) and unwraps a
   * `{jsonrpc, id, result}` envelope defensively in case a future
   * Go server release returns the wrapped shape.
   */
  callTool<T = unknown>(name: string, args: unknown): Observable<T>;

  /**
   * Stream a long-running tool's incremental results via SSE. The
   * Observable emits each parsed `data:` frame as a `T`; the
   * stream completes when the server closes the connection.
   *
   * The current Go server emits `ready` + `ping` heartbeat frames;
   * PR 5+ wires per-execution channels. The adapter is shaped to
   * handle both without changes.
   */
  streamTool<T = unknown>(name: string, args: unknown): Observable<T>;
}