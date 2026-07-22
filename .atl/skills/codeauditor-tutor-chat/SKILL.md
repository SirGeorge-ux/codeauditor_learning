---
name: codeauditor-tutor-chat
description: Use this skill when an agent needs to interact with the tutor chat or design the chat's behavior. Trigger phrases include "tutor", "chat", "explícame", "ayuda con el challenge", "dame una pista", "socratic", "no entiendo por qué falla", "pista", "hint". Handles context awareness, socratic levels, memory, and MCP tool usage.
---

# Tutor Chat (S2+)

> **Outcome:** the tutor chat responds appropriately to the user's message, considering their current context, learning profile, and socratic level.
> **Latency target:** <3s for first token.
> **Memory:** session (10 messages) + persistent (`LearningProfile`).

## When to load

Load this skill when:
- The user sends a message to the tutor chat.
- The agent is designing/improving the tutor chat.
- The agent is debugging a chat issue.
- The user asks "¿qué es la IA socrática?" or "cómo funciona el chat".

## The chat is always visible

When the user is in `/dojo/*` or `/practice/*`, the chat panel is on the right (desktop) or a FAB that opens a modal (mobile).

## Context awareness

The chat ALWAYS has access to this context (invisible to the user):

```typescript
interface ChatContext {
  user: {
    id: string;
    email: string;
    rango_global: 'Junior' | 'Mid' | 'Senior' | 'Architect';
    racha_dias: number;
  };

  language: {
    current: string;                              // 'typescript', 'rust', etc.
    rango_in_language: 'F' | 'E' | 'D' | 'C' | 'B' | 'A' | 'S';
    puntos: number;
    tasa_exito: number;
  };

  location: {
    route: string;                                // '/dojo/ch-sqli', '/practice/free/typescript', etc.
    challenge?: {
      id: string;
      title: string;
      difficulty: string;
      code: string;                               // current Monaco content
      output?: string;                            // last sandbox output
    };
  };

  history: {
    recent_challenges: string[];                  // last 5 challenge IDs
    hints_used_recently: number[];                // last 3 sessions
    common_blocks: string[];                      // from LearningProfile
  };

  preferences: {
    idioma: 'es' | 'en' | 'fr' | 'de';
    nivel_socratismo: 0 | 1 | 2 | 3;              // default 2
    longitud_maxima_msg: number;                  // palabras
  };
}
```

This context is sent in every LLM call as `system` messages + the actual user message.

## Socratic levels (4)

The chat uses `mcp-pedagogical.get_socratic_prompt(level, language, topic, user_message)` to load the right system prompt.

| Level | Comportamiento | Prompt file | Cuándo se usa |
|---|---|---|---|
| 0 (Directa) | "Hay un SQL injection en línea 3, usa queries parametrizadas" | `.atl/prompts/tutor-socratic-0.md` | Rango Senior+ que ya falló 2 veces, opt-in |
| 1 (Guiada) | "Mira la línea 3. ¿Qué pasa si `username = \"' OR 1=1--\"`?" | `.atl/prompts/tutor-socratic-1.md` | Default para rango Mid |
| 2 (Socrática pura) | "¿Qué tipo de dato es `username`? ¿Y si el usuario lo controla?" | `.atl/prompts/tutor-socratic-2.md` | Default para rango Junior |
| 3 (Descubrimiento) | La IA NO responde. Espera hipótesis del user. | `.atl/prompts/tutor-socratic-3.md` | Modo avanzado opt-in |

**Default per range:**

| Rango free practice | Rango code health | Socratismo default |
|---|---|---|
| F-E | Junior | 2 (pura) |
| D-C | Mid | 1 (guiada) |
| B-A | Senior | 0 (directa) si falla 2 veces |
| S | Architect | 0 (directa) |

**Cómo cambiar el nivel:**

- En el perfil: `LearningProfile.nivel_socratismo`.
- En el chat: comandos `/nivel 0`, `/nivel 1`, `/nivel 2`, `/nivel 3`.
- Auto-escalada: si el user falla 2 veces en el mismo challenge, sube un nivel de directness.

## Memory

### Short-term (session)

- Ventana de **10 mensajes** de la conversación actual.
- Se envía al LLM en cada llamada.
- Se descarta al cerrar el chat o al cambiar de challenge.

### Long-term (persistent)

- **`LearningProfile`** en la DB:
  - Preferencias (idioma, socratismo, longitud).
  - Estilo de aprendizaje.
  - Bloqueos típicos.
  - Resumen ("Sabe TS junior, le cuestan genéricos, prefiere ejemplos").
- **Se actualiza** después de cada audit completado.
- **Se usa** para personalizar el system prompt del chat.

### Resumen automático

Cada 10 mensajes, el chat genera un resumen y lo guarda en `LearningProfile.resumen`. Usa el LLM con un prompt específico:

```python
prompt = f"""
Resume la siguiente conversación del tutor con el user:

{messages}

Genera un resumen de 2-3 frases en español que capture:
- Qué ha aprendido el user en esta sesión
- Dónde se ha atascado
- Cómo ha respondido al socratismo
"""
summary = llm.generate(prompt)
update_learning_profile(user_id, summary=summary)
```

## MCP tools que el chat usa

| User dice | Chat usa | Tool MCP |
|---|---|---|
| "Dame un ejercicio" | Lista topics del nivel del user | `mcp-curriculum.get_topics` |
| "Dame uno sobre Herencias" | Genera el challenge | `mcp-pedagogical.get_learning_objective` + `mcp-challenges.generate_challenge` |
| "Otro más difícil" | Sube nivel y regenera | `mcp-pedagogical.get_learning_objective(lang, topic, level+1)` |
| "No entiendo por qué falla" | Análisis socrático del código | `mcp-pedagogical.get_socratic_prompt` + análisis del código |
| "¿Qué es Herencia?" | Mini-lección + ejercicio corto | `mcp-curriculum.get_topic` + `mcp-pedagogical.generate_story_context` |
| "¿Cómo se usa X función de Y librería?" | Docs oficiales actualizadas | `mcp-external-docs.get_library_docs` |
| "Muéstrame la solución" | Revela `solutionCode` con penalización | `mcp-challenges.get_challenge` (con `reveal_solution=true`) |
| "¿Cómo voy?" | Resumen del `LanguageProgress` | `mcp-user.get_language_progress` |
| "Estoy aburrido" | Sugiere otro smell o cambio de lenguaje | `mcp-user.get_learning_profile` + `mcp-curriculum.get_topics` |

## System prompt structure

```python
system_messages = [
    # 1. Identidad del tutor
    {"role": "system", "content": load_prompt("tutor-system.md")},
    
    # 2. Nivel socrático
    {"role": "system", "content": load_prompt(f"tutor-socratic-{level}.md")},
    
    # 3. Contexto del user
    {"role": "system", "content": format_user_context(context.user)},
    
    # 4. Contexto del lenguaje
    {"role": "system", "content": format_language_context(context.language)},
    
    # 5. Contexto de la ubicación (challenge actual)
    {"role": "system", "content": format_location_context(context.location)},
    
    # 6. Historial reciente
    {"role": "system", "content": format_history(context.history)},
    
    # 7. Preferencias
    {"role": "system", "content": format_preferences(context.preferences)},
    
    # 8. Tools disponibles
    {"role": "system", "content": list_available_tools(tools)},
    
    # 9. Mensajes de la sesión
    {"role": "user", "content": previous_messages},
    
    # 10. Mensaje actual
    {"role": "user", "content": current_message},
]
```

## Restricciones del chat (CRÍTICO)

- **NO** resuelve el challenge por el user. Si pide "dame el código de la solución", el chat le da pistas graduadas.
- **NO** habla de otros users. Single-tenant.
- **NO** cambia de tema si el user está en pleno challenge. Sugiere "termina este y luego vemos".
- **NO** usa socratismo nivel 0 (directa) sin que el user haya fallado 2 veces o sea opt-in.
- **SÍ** puede usar todas las MCP tools.
- **SÍ** mantiene memoria de la conversación actual.
- **SÍ** actualiza `LearningProfile` periódicamente.

## Auto-escalada de socratismo

Si el user:
- Falla el mismo challenge 2 veces → sube un nivel de directness.
- Pide "dame la respuesta" 3 veces → sube 2 niveles.
- Está atascado >10 min → el chat pregunta "¿quieres una pista?" (no la da sin pedir).

Si el user:
- Resuelve el challenge con hints 1-2 → no cambia nivel.
- Resuelve sin hints → sube un nivel de socratismo (más pregunta, menos respuesta).

## Comandos del chat

El chat entiende comandos especiales:

| Comando | Efecto |
|---|---|
| `/nivel 0-3` | Cambia el nivel de socratismo |
| `/pista` | Da la siguiente pista (consume puntos si no es la 1) |
| `/solucion` | Muestra la solución (penalización alta) |
| `/reset` | Borra la memoria de la sesión actual |
| `/perfil` | Muestra el `LanguageProgress` |
| `/diccionario <termino>` | Busca en el glosario |
| `/help` | Lista los comandos |

## Don't do

- ❌ Don't solve the challenge. The user has 3 hints for that.
- ❌ Don't use socratismo 0 by default. Only after failures.
- ❌ Don't break the socratic level (e.g. respond directly when level is 2).
- ❌ Don't ignore the user's preferences (idioma, longitud).
- ❌ Don't make the chat too long. Truncate at `longitud_maxima_msg` words.
- ❌ Don't expose the internal prompt structure to the user.
- ❌ Don't make up challenges that don't exist in the curriculum.

## Resources

- **Spec:** `openspec/changes/tutor-chat/`
- **Doc:** `docs/business-domain.md` §4 (Tutor socrático)
- **Doc:** `docs/mcp-tools.md` §3.6 (`mcp-pedagogical`)
- **Doc:** `docs/llm-strategy.md` (cascade)
- **Skill:** `codeauditor-free-practice-generator` (cómo el chat genera challenges)
- **Harness:** `add-tutor-prompt.harness.md` (cómo añadir/iterar un prompt)
