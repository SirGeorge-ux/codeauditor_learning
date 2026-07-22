# LLM Strategy (CodeAuditor)

> **Audiencia:** devs, tech leads, agentes que vayan a tocar el sistema de IA.
> **Lee esto antes de:** añadir un proveedor de LLM, cambiar el cascade, optimizar costes, o tocar el `MCP monolítico`.

---

## 1. Principio rector

**Por defecto, todo el LLM es GRATIS (Ollama local o OpenRouter free tier). El pago es la excepción, no la norma. Y MiniMax M3 es OPT-IN, no default.**

Razones:
- El proyecto es personal (single-user, `ggogsmic`). No hay justificación para gastar.
- Los modelos gratis actuales (Qwen 2.5 Coder 32B, Llama 3.3 70B, DeepSeek V3) son **comparables o mejores** que MiniMax M3 en tareas técnicas.
- Si el cascade gratuito falla 3 veces seguidas, el sistema pregunta al user antes de gastar.

---

## 2. Las 3 puertas de LLM

### 2.1 Ollama local (default para audit + code health local)

- **Modelo:** `qwen2.5-coder:3b` (3B parámetros).
- **Cuándo:** audit en sandbox, análisis de archivos individuales en code health, validación de SQL.
- **Costo:** $0 (corre en la máquina del user).
- **Pros:** privacidad, velocidad, sin rate limit, sin internet.
- **Contras:** capacidad limitada (3B params). Socratismo débil. Contexto corto.
- **Por qué este modelo:** corre en CPU modesta, 1-2GB RAM, suficiente para análisis de snippets.

### 2.2 OpenRouter cascade (default para generación + chat)

- **URL:** `https://openrouter.ai/api/v1`
- **Costo:** $0 (todos los tier 1 y tier 2 son free tier en OpenRouter).
- **Tier 1 — Gratis rápido (default):**

| Modelo | Provider | Mejor para | Costo |
|---|---|---|---|
| `llama-3.3-70b-versatile` | Groq | Razonamiento, chat socrático, generación de challenges | $0 |
| `qwen-2.5-coder-32b` | Groq | Análisis de código, code review, generación de snippets | $0 |
| `llama-3.3-70b` | Cerebras | Fallback rápido cuando Groq está saturado | $0 |

- **Tier 2 — Gratis potente (cuando tier 1 no llega):**

| Modelo | Provider | Mejor para | Costo |
|---|---|---|---|
| `deepseek-chat` (DeepSeek V3) | DeepSeek | Contexto largo (análisis de repo entero), razonamiento | $0 |
| `qwen-2.5-72b` | (varios) | Alternativa tier 2 | $0 |

- **Tier 3 — Pago barato (último recurso, requiere opt-in del user):**

| Modelo | Provider | Mejor para | Costo |
|---|---|---|---|
| `gpt-4o-mini` | OpenAI | Fallback pago si los gratis fallan | ~$0.15/1M tokens |

### 2.3 MiniMax M3 (opt-in premium)

- **Cuándo:** solo si el user lo activa explícitamente en su perfil (toggle).
- **Por qué opt-in y no default:** los tier 1 y tier 2 son **iguales o mejores** en tareas técnicas. M3 no aporta valor extra si los gratis funcionan.
- **Excepción:** el user tiene plan MiniMax MAX con muchos tokens. Si quiere respuestas "premium", activa M3. Pero por defecto, gratis.
- **Config:** `MINIMAX_BASE_URL` + `MINIMAX_API_KEY` en `.env` del backend.

---

## 3. Distribución por caso de uso

| Caso de uso | Puerta | Modelo default | Costo | Latencia |
|---|---|---|---|---|
| Audit en sandbox (análisis) | Ollama local | `qwen2.5-coder:3b` | $0 | ~1s/archivo |
| Code health (1 archivo) | Ollama local | `qwen2.5-coder:3b` | $0 | ~2s/archivo |
| Code health (repo entero) | OpenRouter | `deepseek-chat` | $0 (free) | ~30s/repo |
| Generación de challenges (free practice) | OpenRouter | `qwen-2.5-coder-32b` | $0 (free) | ~5s/challenge |
| Tutor chat (socrático) | OpenRouter | `llama-3.3-70b-versatile` | $0 (free) | ~2s/msg |
| Análisis profundo de repo (raro) | OpenRouter | `deepseek-chat` | $0 (free) | ~60s |
| Fallback pago | OpenRouter | `gpt-4o-mini` | ~$0.15/1M | ~3s |
| Premium opt-in | MiniMax | M3 | plan MiniMAX | ~3s |

---

## 4. Cascade logic

```
Request LLM
   ↓
[1] ¿Es audit en sandbox o code health de 1 archivo?
   ├─ SÍ → Ollama local
   └─ NO ↓
[2] ¿El user activó "premium M3" en su perfil?
   ├─ SÍ → MiniMax M3
   └─ NO ↓
[3] Cascade OpenRouter (tier 1 → tier 2 → tier 3)
   ├─ [3a] Groq llama-3.3-70b-versatile (chat, razonamiento)
   ├─ [3b] Groq qwen-2.5-coder-32b (código)
   ├─ [3c] Cerebras llama-3.3-70b (fallback rápido)
   ├─ [3d] DeepSeek V3 (contexto largo)
   └─ [3e] GPT-4o-mini (pago, requiere opt-in)
```

**El cascade es configurable por use case** (no es un cascade único para todo):

```yaml
# config/llm-cascade.yaml
routes:
  audit_sandbox:
    provider: ollama
    model: qwen2.5-coder:3b
    base_url: http://localhost:11434

  code_health_local:
    provider: ollama
    model: qwen2.5-coder:3b
    base_url: http://localhost:11434

  code_health_repo:
    cascade:
      - provider: openrouter
        model: deepseek-chat
        tier: 2
      - provider: openrouter
        model: gpt-4o-mini
        tier: 3
        requires_opt_in: true

  challenge_generation:
    cascade:
      - provider: openrouter
        model: qwen-2.5-coder-32b
        tier: 1
      - provider: openrouter
        model: llama-3.3-70b-versatile
        tier: 1
      - provider: ollama
        model: qwen2.5-coder:3b
        tier: 0

  tutor_chat:
    cascade:
      - provider: openrouter
        model: llama-3.3-70b-versatile
        tier: 1
      - provider: openrouter
        model: deepseek-chat
        tier: 2

  fallback_paid:
    provider: openrouter
    model: gpt-4o-mini
    requires_opt_in: true
    monthly_cap_usd: 5.00
```

---

## 5. Por qué NO fijar M3

| Argumento "a favor de M3 fijo" | Contraargumento |
|---|---|
| "Es el que tengo contratado" | Tienes plan, no obligación de usarlo para todo. El plan tiene muchos tokens, pero los tier 1 son gratis. |
| "Es el más capaz" | En código, Qwen 2.5 Coder 32B le iguala o supera. En razonamiento, Llama 3.3 70B le iguala. |
| "Quiero consistencia" | El cascade da consistencia por use case. Cada caso usa siempre el mismo modelo. |
| "Es seguro" | OpenRouter es seguro. Usa HTTPS, API keys en .env, sin logs. |
| "El setup es más simple" | OpenRouter es UNA puerta. El setup es igual de simple. |

**Conclusión:** M3 queda como opt-in. Si el user lo quiere siempre, lo activa. Si no, el cascade gratis le da el 99% del valor al 0% del costo.

---

## 6. Context7 (complemento para docs de librerías)

- **Qué:** servidor MCP de Upstash que da docs actualizadas de 5.000+ librerías.
- **Cuándo:** cuando el chat del tutor necesita responder "¿cómo se usa X función de Y librería?" con docs oficiales actualizadas.
- **Cómo:** wrapper en `infrastructure/mcp/external_docs/` (mcp-external-docs).
- **Tool principal:** `get_library_docs(library, topic?)`.
- **Trampa:** Context7 NO da temario pedagógico. Solo docs. Para "qué enseñar primero" usamos nuestra currícula.

---

## 7. Brave Search (complemento para búsqueda general)

- **Cuándo:** cuando Context7 no tiene la librería o el user pregunta algo no documentado.
- **Cómo:** wrapper en `infrastructure/mcp/external_docs/`.
- **Tool principal:** `search_documentation(query)`.
- **Costo:** plan free limitado (1-2K queries/mes), plan de pago razonable.

---

## 8. Estructura de costos esperada (mensual, usuario activo)

| Operación | Frecuencia | Costo unitario | Costo mensual |
|---|---|---|---|
| Audit local | 100/mes | $0 (Ollama) | **$0** |
| Code health (manual) | 5/mes | $0 (Ollama) | **$0** |
| Code health (repo entero, raro) | 1/mes | $0 (DeepSeek free) | **$0** |
| Generación de challenges | 50/mes | $0 (Groq free) | **$0** |
| Tutor chat | 200 msgs/mes | $0 (Groq free) | **$0** |
| Fallback pago (raro) | 1/mes | $0.05 | **$0.05** |
| **Total estimado** | | | **~$0.05/mes** |

Si activas M3 para todo: ~$5-10/mes.

**Cap mensual configurable:** el user puede poner `monthly_cap_usd: 5.00` en config. Si se pasa, cascade para y avisa.

---

## 9. Trampas y mitigaciones

| Trampa | Mitigación |
|---|---|
| OpenRouter rate limit en tier free | Cascade con Cerebras como fallback. Si ambos fallan, deepseek. |
| Ollama local saturado | Si la máquina tiene <8GB RAM, Ollama compite con el navegador. Cachear resultados, reducir concurrencia. |
| M3 opt-in activado por error | Confirmación al activar. Toggle visible en perfil. |
| Coste inesperado | Cap mensual + alerta al 80%. Logs de gasto. |
| LLM alucina ejercicio no resoluble | Bucle de validación: generar → ejecutar en sandbox → si falla, regenerar con feedback. |
| LLM da respuesta spoiler en modo socrático | Validar el prompt antes de enviar. Si nivel socratismo=2 y la respuesta empieza con "El problema es X", regenerar. |
| API keys filtradas al frontend | Backend como único punto de contacto. El frontend NUNCA llama a OpenRouter/MiniMax directamente. |

---

## 10. Variables de entorno

```bash
# .env del backend

# Ollama local
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=qwen2.5-coder:3b

# OpenRouter
OPENROUTER_API_KEY=sk-or-...
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1

# MiniMax (opt-in)
MINIMAX_BASE_URL=https://api.minimaxi.chat/v1
MINIMAX_API_KEY=eyJ...

# Context7
CONTEXT7_API_KEY=ctx7_...

# Brave Search
BRAVE_API_KEY=BSA...

# Caps
LLM_MONTHLY_CAP_USD=5.00
```

**Regla:** NUNCA commitear `.env` con valores reales. Solo `.env.example` con placeholders.

---

## 11. Métricas a trackear

| Métrica | Dónde | Para qué |
|---|---|---|
| Latencia p50/p95 por provider | logs | Detectar providers lentos |
| Tasa de fallback (cuándo tier 1 falla) | logs | Ajustar cascade |
| Coste mensual por provider | logs | Control de gasto |
| % de ejercicios generados que compilan | `mcp-pedagogical.validate_solution` | Calidad del generador |
| % de respuestas socráticas que NO spoilean | `mcp-pedagogical.get_socratic_prompt` + validación | Calidad pedagógica |

---

## 12. Recursos

- **OpenRouter docs:** https://openrouter.ai/docs
- **Ollama docs:** https://ollama.com/docs
- **Context7:** https://context7.com
- **MiniMax API:** https://api.minimaxi.chat/docs
- **Spec futuro:** `openspec/changes/llm-cascade/`
- **Skill:** `codeauditor-free-practice-generator` (cómo pedir al LLM un challenge)
- **Skill:** `codeauditor-tutor-chat` (cómo configurar el socratismo)
