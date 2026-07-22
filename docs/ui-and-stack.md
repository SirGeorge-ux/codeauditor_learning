# UI/UX y Stack Frontend (CodeAuditor)

> **Audiencia:** devs frontend, diseñadores, agentes que toquen componentes.
> **Lee esto antes de:** crear componentes, modificar estilos, añadir páginas, o cambiar el tema.

---

## 1. Principios visuales

- **Dark IDE / Cyber-Minimalista** — el frontend evoca un IDE moderno (VSCode, HackTheBox, GitHub Dark).
- **Densidad media** — no es un landing page. Es un espacio de trabajo. Las cards tienen padding generoso pero el contenido se ve denso.
- **Contraste fuerte** — el texto secundario es legible pero no compite con el principal.
- **Color = significado** — verde = OK, rojo = error/vuln, amarillo = warning/code smell. No decorativos.

---

## 2. Paleta (declarada en `styles.css` con `@theme {}`)

```css
@theme {
  --color-dojo-base:    #0d1117;   /* fondo principal */
  --color-dojo-surface: #161b22;   /* paneles, cards, sidebar */
  --color-dojo-text:    #c9d1d9;   /* texto principal */
  --color-dojo-error:   #f85149;   /* rojo carmesí */
  --color-dojo-accent:  #39d353;   /* verde neón */
  --color-dojo-warning: #d29922;   /* amarillo */
  --color-dojo-border:  #30363d;   /* bordes sutiles */
}
```

**Reglas:**
- **No** usar colores fuera de esta paleta sin justificación.
- **No** hardcodear hex en componentes (`text-[#39D353]` está OK para casos puntuales, pero prefiere `text-dojo-accent` cuando se pueda).
- **No** crear un `tailwind.config.js` — la config es CSS-first con `@theme {}`.

---

## 3. Tipografía

- **Familia base:** `system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`.
- **Mono (en panels de código):** `monospace` (Tailwind `font-mono`).
- **Tamaños comunes:** `text-xs` (12px), `text-sm` (14px), `text-base` (16px), `text-lg` (18px), `text-xl` (20px), `text-3xl` (30px — para stats), `text-5xl` (48px — para hero).
- **Peso:** `font-medium` (500) en botones y labels, `font-bold` (700) en stats y CTAs, `font-normal` (400) en body.

---

## 4. Iconos

- **Lucide Angular** (`@lucide/angular`).
- Uso: `<lucide-icon name="play" class="w-4 h-4"></lucide-icon>`.
- Tamaño: 4×4 (16px) en botones, 5×5 (20px) en headers, 6×6 (24px) en empty states.
- **No** mezclar librerías de iconos (no Material Icons, no FontAwesome, no Heroicons).

---

## 5. Layout principal

El layout autenticado usa `MainLayoutComponent` con:

```
┌─────────────────────────────────────────────────────┐
│  Sidebar (colapsable)  │   Router outlet           │
│  - Logo                │                            │
│  - Nav items           │   (Dashboard, Dojo, etc.)  │
│  - User profile        │                            │
└─────────────────────────────────────────────────────┘
```

- **Sidebar:** `bg-dojo-surface`, ancho fijo de 240px expandido, 64px colapsado. Transición suave.
- **Top bar (si existe):** sticky, fondo `bg-dojo-base` con border-bottom `border-dojo-border`.
- **Contenido:** padding `p-6` o `p-8` según densidad.

---

## 6. Componentes compartidos (en `infrastructure/components/shared/`)

| Componente | Selector | Propósito |
|---|---|---|
| `code-panel.component.ts` | `app-code-panel` | Wrapper de Monaco Editor. Recibe código + lenguaje. |
| `context-panel.component.ts` | `app-context-panel` | Panel lateral con info del challenge (título, dificultad, descripción). |
| `terminal-panel.component.ts` | `app-terminal-panel` | Wrapper de xterm.js para la salida del sandbox. |
| `resize.directive.ts` | `[appResize]` | Directiva para hacer splitters redimensionables. |

**Regla:** si un componente es reusable, va en `shared/`. Si es de una página, va en `<página>/`.

---

## 7. Páginas y rutas (estado actual + roadmap)

### 7.1 Rutas existentes (S0)

| Ruta | Componente | Auth | Descripción |
|---|---|---|---|
| `/` | `HomeComponent` (`ui/`) | No | Landing con CTA a login/register + 3 modos de práctica (S2) |
| `/login` | `LoginComponent` | No | Login con Supabase |
| `/register` | `RegisterComponent` | No | Registro |
| `/dashboard` | `DashboardPageComponent` | Sí | Grid de challenges (curated) + stats del usuario |
| `/dojo` | `DojoPageComponent` | Sí | Workspace vacío (para challenges manuales) |
| `/dojo/:id` | `DojoPageComponent` | Sí | Workspace con challenge cargado (curated o importado) |
| `/mcp` | `McpPageComponent` | Sí | Explorador de repos Gogs + importador de archivos (legacy) |
| `/vault` | `VaultPageComponent` | Sí | Historial de auditorías + stats |
| `**` | redirect a `/dashboard` | — | Fallback |

**Pendiente S0:** las rutas autenticadas **deberían usar `loadComponent`** para lazy loading. Ver H-14 de la auditoría.

### 7.2 Rutas nuevas (S1-S4)

| Ruta | Sprint | Componente | Auth | Descripción |
|---|---|---|---|---|
| `/practice` | S2 | `PracticePageComponent` | Sí | Hub de los 3 modos (curated, free, code-health) |
| `/practice/free` | S3 | `FreePracticePageComponent` | Sí | Lista de lenguajes + generador de challenges on-demand |
| `/practice/free/:lang` | S3 | `FreePracticeLangPageComponent` | Sí | Challenge generado para ese lenguaje |
| `/repo/:owner/:name/code-health` | S4 | `CodeHealthPageComponent` | Sí | Dashboard de análisis del repo |
| `/profile` | S1 | `ProfilePageComponent` | Sí | LanguageProgress + racha + gráfico radar + opt-in M3 |
| `/dictionary` | S2 | `DictionaryPageComponent` | Sí | Glosario personal + búsqueda + añadir |
| `/dictionary/:term` | S2 | `DictionaryTermPageComponent` | Sí | Detalle de un término |
| `/curriculum/review` | S2 | `CurriculumReviewPageComponent` | Sí | Peer review de topics propuestos por la IA |

### 7.3 El Tutor Chat (S2) — siempre visible

El `TutorChatPanelComponent` aparece automáticamente como **panel lateral colapsable a la derecha** en todas las rutas bajo `/dojo/*` y `/practice/*`. En pantallas <1024px se transforma en un **FAB** que abre un modal.

```
Desktop:
┌─────────────────────────────────────┬──────────────┐
│                                     │  💬 Tutor    │
│   [Dojo / Free Practice]            │ ─────────── │
│                                     │ Hola, mic.   │
│                                     │ Veo que      │
│                                     │ estás en     │
│                                     │ ch-sqli.     │
│                                     │ ¿Necesitas   │
│                                     │ ayuda?       │
│                                     │              │
│                                     │ [Escribe...] │
└─────────────────────────────────────┴──────────────┘

Mobile (FAB):
┌─────────────────────────┐
│  [Dojo / Free Practice] │
│                         │
│                         │  [💬]  ← FAB
│                         │
└─────────────────────────┘
```

---

## 8. El Dojo (la página más importante)

Layout 3-panel redimensionable:

```
┌──────────────────────────────────────────────────────┐
│ Header: título del challenge + acciones (Run, Reset) │
├──────────────┬──────────────────────┬────────────────┤
│              │                      │                │
│  Context     │  Monaco Editor       │  Terminal      │
│  Panel       │  (código)            │  (xterm)       │
│              │                      │                │
│  - descripción │                    │  Output + IA   │
│  - codeSmell │                      │                │
│  - repoUrl   │                      │                │
│              │                      │                │
└──────────────┴──────────────────────┴────────────────┘
```

- **Splitters** con `appResize`.
- **Botón "Run Audit"** en el header → `POST /api/v1/audit` → SSE stream → terminal output + Ollama tokens.
- **Botón "Reset"** recarga el código original.

---

## 9. Patrones recurrentes

### 9.1 Loading

```html
@if (loading()) {
  <div class="flex items-center gap-2 text-dojo-text/60">
    <lucide-icon name="loader" class="w-4 h-4 animate-spin"></lucide-icon>
    <span>Cargando...</span>
  </div>
}
```

### 9.2 Empty state

```html
<div class="flex flex-col items-center justify-center p-12 text-center">
  <lucide-icon name="folder-open" class="w-12 h-12 text-dojo-text/40"></lucide-icon>
  <p class="mt-4 text-dojo-text/60">No hay challenges todavía</p>
</div>
```

### 9.3 Error inline

```html
<div class="border border-dojo-error bg-dojo-error/10 text-dojo-error p-3 rounded">
  {{ errorMessage() }}
</div>
```

### 9.4 Stat (número grande en verde)

```html
<div class="text-3xl font-bold text-dojo-accent">{{ value() }}</div>
<div class="text-xs text-dojo-text/60">{{ label }}</div>
```

### 9.5 Card de challenge

```html
<article class="border border-dojo-border bg-dojo-surface rounded-lg p-4 hover:border-dojo-accent/50 transition">
  <h3 class="text-lg font-medium text-dojo-text">{{ challenge.title }}</h3>
  <p class="text-sm text-dojo-text/60 mt-1">{{ challenge.codeSmell }}</p>
  <div class="flex gap-2 mt-3 text-xs">
    <span class="px-2 py-0.5 rounded border border-dojo-border">{{ challenge.difficulty }}</span>
    <span class="px-2 py-0.5 rounded border border-dojo-border">{{ challenge.language }}</span>
  </div>
</article>
```

---

## 10. Angular 21: reglas específicas

| Regla | Por qué |
|---|---|
| `bootstrapApplication(AppComponent, appConfig)` en `main.ts` | No más `platformBrowserDynamic` ni `AppModule`. |
| `standalone: true` en todo `@Component` | Por defecto en Angular 19+, explícito en este proyecto. |
| **Prohibido:** `*ngIf`, `*ngFor`, `*ngSwitch` | Usar `@if`, `@for`, `@switch`. |
| **Prohibido:** `NgModule` | Si necesitas "agrupar", usa un archivo barrel (`index.ts`). |
| `inject()` preferentemente sobre constructor DI | Más limpio, mejor tree-shaking. |
| `signal()`, `computed()`, `effect()` para estado local | Más simple que `BehaviorSubject` para el 90% de casos. |
| RxJS solo donde hay streams reales (HTTP, SSE, WebSocket) | No mezclar `subscribe()` con signals sin razón. |
| `track` en `@for` obligatorio | Performance. |

---

## 11. Tailwind 4: reglas específicas

- La config está en `styles.css` con `@theme {}`. **No** crear `tailwind.config.js`.
- Para colores del tema: `bg-dojo-base`, `text-dojo-text`, `border-dojo-border`, etc.
- Para colores arbitrarios puntuales: `text-[#39D353]` está OK, pero prefiere los tokens.
- **No** usar `@apply` abusivamente — el CSS-first de Tailwind 4 es para que el CSS viva en el CSS, no en `@apply` dentro de los componentes.
- Plugin de Vite (`@tailwindcss/vite`) compila on-demand.

---

## 12. Testing visual

- **Storybook:** no instalado. Recomendación: instalarlo en la fase 2 de mejoras.
- **Visual regression:** no instalado. Si se quiere añadir, **Chromatic** o **Percy** con Playwright.
- **Manual:** `pnpm start` y revisar cada página en pantallas de 1280×800, 768×1024, 375×667.

---

## 13. Recursos

- **Spec activo del layout:** `openspec/specs/layout/spec.md`
- **Spec archivado del Dojo original:** `openspec/changes/archive/2026-06-12-dojo-layout/`
- **Skill de cómo añadir un componente compartido:** (crear)
- **Harness de cómo añadir una página nueva:** (crear)
- **Harness:** `.atl/harnesses/add-api-endpoint.harness.md` (también cubre páginas nuevas)
- **Skill:** `codeauditor-tutor-chat` (cómo interactúa el chat con la UI)
