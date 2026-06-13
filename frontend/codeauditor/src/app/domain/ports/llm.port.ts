/**
 * LLMPort — interface for interacting with the LLM (Ollama).
 *
 * The application layer calls this to get explanations and summaries
 * for audit findings. The infrastructure layer provides the Ollama adapter.
 */
export interface LLMPort {
  explainFinding(findingId: string, context: string): Promise<string>;
  summarizeSession(sessionId: string): Promise<string>;
  streamTokens(prompt: string, onToken: (token: string) => void): Promise<void>;
}