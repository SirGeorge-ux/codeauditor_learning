// Ollama adapter — implements LLMPort using Ollama API.
// This is infrastructure (driven/secondary) — depends on domain ports.
import { LLMPort } from "../domain/ports/llm.port";

/**
 * OllamaAdapter — implements LLMPort by calling the local Ollama service.
 * Default endpoint: http://localhost:11434
 */
export class OllamaAdapter implements LLMPort {
  constructor(private readonly baseUrl: string = "http://localhost:11434") {}

  async explainFinding(findingId: string, context: string): Promise<string> {
    // Stub — real implementation calls Ollama /api/generate
    return `[LLM explanation stub for finding ${findingId}]: ${context}`;
  }

  async summarizeSession(sessionId: string): Promise<string> {
    // Stub
    return `Summary stub for session ${sessionId}`;
  }

  async streamTokens(prompt: string, onToken: (token: string) => void): Promise<void> {
    // Stub — real implementation calls Ollama /api/generate with stream:true
    onToken("[stub token]");
  }
}