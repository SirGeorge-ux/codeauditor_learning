-- Seed: Replace 8 curated challenges with v2 format (no-spoiler descriptions,
-- structured hints, expected findings, test cases, linter rules, solution code).
--
-- Uses ON CONFLICT (id) DO UPDATE so existing rows from 003_seed_challenges.sql
-- are upgraded in place. Idempotent: can be re-run safely.
--
-- All descriptions describe context ONLY — the name of the code smell is never
-- revealed, preserving the audit experience.

-- =============================================================================
-- 1. ch-sqli — SQL Injection (TypeScript, security, junior)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-sqli',
    'Autenticación con Consulta Dinámica',
    'Este endpoint de autenticación recibe credenciales y construye una consulta SQL dinámica para validar al usuario. En pruebas locales todo funciona. Investiga el código y encuentra los problemas de seguridad y mantenibilidad.',
    'junior', 'security', 'typescript', 'https://github.com/example/vulnerable-api',
    $code$insert { db } from './database';

export async function login(username: string, password: string) {
  const query = `SELECT * FROM users WHERE username = '${username}' AND password = '${password}'`;
  const user = await db.execute(query);

  if (user.rows.length > 0) {
    return { success: true, token: generateToken(user.rows[0]) };
  }

  return { success: false };
}$code$,
    'SQL Injection',
    'available', '2025-01-01T00:00:00Z',
    '["Entender por qué concatenar inputs en SQL es peligroso", "Usar consultas parameterizadas"]'::jsonb,
    '["Confiar en que el frontend valida los inputs", "Usar REPLACE o blacklist de caracteres", "Pensar que un ORM elimina el riesgo si igual se concatenan strings"]'::jsonb,
    '[
      {"level": 1, "content": "Mira cómo se construye la variable query. ¿Qué pasa si username contiene una comilla simple?", "cost_points": 0},
      {"level": 2, "content": "Existe una forma de pasar valores al driver de la base de datos sin concatenar strings. ¿La conoces?", "cost_points": 10},
      {"level": 3, "content": "La solución es usar parámetros posicionales: db.execute(text, valores) en vez de construir el string a mano.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "security", "message": "La consulta SQL se construye concatenando inputs del usuario, permitiendo inyección", "evidence": "const query = `SELECT * FROM users WHERE username = ''${username}''`", "suggested_fix": "Usar consultas parameterizadas: SELECT * FROM users WHERE username = $1 AND password = $2 con db.query(text, [username, password])"},
      {"severity": "medium", "category": "security", "message": "La contraseña se compara en texto plano", "evidence": "AND password = ''${password}''", "suggested_fix": "Almacenar y comparar hashes (bcrypt/argon2) en lugar de texto plano"}
    ]'::jsonb,
    '[
      {"name": "Login con credenciales válidas devuelve success", "input": {"username": "admin", "password": "secret"}, "expected_output": {"success": true}, "weight": 3},
      {"name": "Login con credenciales inválidas devuelve failure", "input": {"username": "admin", "password": "wrong"}, "expected_output": {"success": false}, "weight": 3},
      {"name": "Input malicioso no devuelve success", "input": {"username": "'' OR ''1''=''1", "password": "anything"}, "expected_output": {"success": false}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "no-string-concat-in-query", "config": {"severity": "error"}}
    ]'::jsonb,
    $code$import { db } from './database';

export async function login(username: string, password: string) {
  const user = await db.query(
    'SELECT * FROM users WHERE username = $1 AND password_hash = $2',
    [username, hashPassword(password)]
  );

  if (user.rows.length > 0) {
    return { success: true, token: generateToken(user.rows[0]) };
  }

  return { success: false };
}$code$,
    'La consulta usa parámetros posicionales ($1, $2) en lugar de concatenar el input, eliminando el vector de inyección. La contraseña se compara como hash, no como texto plano.',
    100, 40, 0, false, 10, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 2. ch-xss — XSS (TypeScript, security, junior)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-xss',
    'Renderizado de Contenido de Usuarios',
    'Este componente muestra comentarios de usuarios en el DOM. En desarrollo todo se ve bien, pero algunas entradas provocan comportamientos inesperados en el navegador de otros usuarios. Examinar el código y encontrar los problemas.',
    'junior', 'security', 'typescript', 'https://github.com/example/social-app',
    $code$import { Component, Input } from '@angular/core';

@Component({
  selector: 'comment-thread',
  template: '<div [innerHTML]="renderComment()"></div>'
})
export class CommentThread {
  @Input() comments: Comment[] = [];

  renderComment(): string {
    return this.comments
      .map(c => `<div class="comment">${c.body}</div>`)
      .join('');
  }
}$code$,
    'Cross-Site Scripting',
    'available', '2025-01-02T00:00:00Z',
    '["Entender por qué renderizar HTML crudo es peligroso", "Usar sanitización o escaping de contenido dinámico"]'::jsonb,
    '["Confiar en que el backend escapa el contenido", "Pensar que innerHTML en Angular siempre es seguro", "Usar replazar <script> sin cubrir todos los vectores"]'::jsonb,
    '[
      {"level": 1, "content": "Observa cómo se renderiza el contenido del comentario. ¿El navegador lo interpreta como HTML?", "cost_points": 0},
      {"level": 2, "content": "Angular tiene mecanismos para sanitizar HTML dinámico de forma segura. ¿Los estás usando?", "cost_points": 10},
      {"level": 3, "content": "La solución es inyectar DomSanitizer y usar sanitize(BypassType, value) o evitar innerHTML y usar interpolación {{ }}.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "security", "message": "El contenido de usuarios se renderiza como HTML crudo, permitiendo inyección de scripts", "evidence": "template: ''<div [innerHTML]="renderComment()"></div>''", "suggested_fix": "Usar interpolación {{ c.body }} en lugar de innerHTML, o sanitizar con DomSanitizer"},
      {"severity": "medium", "category": "maintainability", "message": "La lógica de renderizado mezcla presentación y string-building", "evidence": ".map(c => `<div class="comment">${c.body}</div>`)", "suggested_fix": "Delegar el renderizado al template de Angular con @for en lugar de construir strings HTML"}
    ]'::jsonb,
    '[
      {"name": "Texto plano se muestra como texto", "input": {"body": "<script>alert(1)</script>"}, "expected_output": {"rendered_as_html": false}, "weight": 5},
      {"name": "Comentario válido se muestra correctamente", "input": {"body": "Hola mundo"}, "expected_output": {"rendered_as_text": true}, "weight": 2},
      {"name": "No se ejecutan scripts embebidos", "input": {"body": "<img src=x onerror=alert(1)>"}, "expected_output": {"script_executed": false}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "no-inner-html", "config": {"severity": "error"}},
      {"type": "angular-security-sanitize", "config": {"enforce": true}}
    ]'::jsonb,
    $code$import { Component, Input } from '@angular/core';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Component({
  selector: 'comment-thread',
  template: `
    @for (c of comments; track c.id) {
      <div class="comment">{{ c.body }}</div>
    }
  `
})
export class CommentThread {
  @Input() comments: Comment[] = [];

  // Si se necesita HTML controlado, sanitizar:
  renderSafe(body: string): SafeHtml {
    return this.sanitizer.sanitize(SecurityContext.HTML, body) ?? '';
  }

  constructor(private sanitizer: DomSanitizer) {}
}$code$,
    'Se reemplaza innerHTML por interpolación {{ c.body }}, que Angular escapa automáticamente. Para HTML controlado, se usaría DomSanitizer.sanitize(). El template usa @for (nuevo control flow) en lugar de construir strings HTML manualmente.',
    100, 40, 0, false, 10, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 3. ch-god — God Function (TypeScript, refactor, mid)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-god',
    'Procesamiento de Pagos Monolítico',
    'Esta función procesa todos los tipos de pago de la plataforma en un solo bloque. Funciona correctamente, pero los desarrolladores la evitan cuando necesitan tocar el código. Cualquier cambio en un tipo de pago puede afectar a los demás. Identifica los problemas de diseño y mantenibilidad.',
    'mid', 'refactor', 'typescript', 'https://github.com/example/payment-service',
    $code$async function handlePayment(order: Order, method: string, data: any) {
  if (method === 'credit_card') {
    const validated = validateCard(data.cardNumber, data.cvv, data.expiry);
    if (!validated) throw new Error('Invalid card');
    const charged = await chargeCard(data.cardNumber, order.total);
    if (!charged) { await notifyFailure(order.userId); return false; }
    await sendEmail(order.userId, 'payment-ok');
    await updateInventory(order.items);
    return true;
  } else if (method === 'paypal') {
    const token = await getPaypalToken(data.code);
    const executed = await executePaypal(token, order.total);
    if (!executed) { await logError('paypal', order.id); return false; }
    await sendEmail(order.userId, 'payment-ok');
    return true;
  } else if (method === 'crypto') {
    const tx = await blockchainTransaction(data.wallet, order.total);
    await waitForConfirmations(tx.hash, 3);
    await sendEmail(order.userId, 'payment-ok');
    return true;
  } else if (method === 'transfer') {
    const bankRef = data.bankRef;
    await verifyTransfer(bankRef, order.total);
    await sendEmail(order.userId, 'payment-ok');
    await sendEmail(order.userId, 'invoice', { ref: bankRef });
    return true;
  }
  throw new Error('Unsupported payment method');
}$code$,
    'God Function',
    'available', '2025-01-03T00:00:00Z',
    '["Identificar el patrón Strategy para reemplazar condicionales", "Separar responsabilidades una por tipo de pago"]'::jsonb,
    '["Dividir en funciones sueltas sin interfaz común", "Usar un switch fuera de la función", "Mover todo a una clase sin reducir el acoplamiento"]'::jsonb,
    '[
      {"level": 1, "content": "Cuenta cuantos tipos de pago hay. ¿Qué pasa con la complejidad si agregas uno más?", "cost_points": 0},
      {"level": 2, "content": "Existe un patrón de diseño que permite reemplazar una cadena de if/else por objetos polimórficos. ¿Lo recuerdas?", "cost_points": 10},
      {"level": 3, "content": "La solución es aplicar Strategy: una interfaz PaymentMethod con un handle por cada tipo, y un registry que selecciona el correcto.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "design", "message": "La función mezcla 4 tipos de pago en un solo cuerpo, imposibilitando testeo aislado", "evidence": "if (method === ''credit_card'') { ... } else if (method === ''paypal'') { ... }", "suggested_fix": "Extraer cada tipo a su propia clase implementando una interfaz PaymentMethod"},
      {"severity": "medium", "category": "design", "message": "El envío de email de confirmación se repite en cada rama", "evidence": "await sendEmail(order.userId, ''payment-ok'')", "suggested_fix": "Centralizar el post-procesamiento común (email, notificación) en un método del PaymentService"},
      {"severity": "low", "category": "maintainability", "message": "El parámetro data es any, sin tipado ni validación de estructura", "evidence": "data: any", "suggested_fix": "Definir tipos discriminated unions por método de pago"}
    ]'::jsonb,
    '[
      {"name": "Procesar pago con tarjeta", "input": {"method": "credit_card", "total": 100}, "expected_output": {"success": true}, "weight": 3},
      {"name": "Procesar pago con PayPal", "input": {"method": "paypal", "total": 50}, "expected_output": {"success": true}, "weight": 3},
      {"name": "Cada handler es testeable de forma aislada", "input": {"check_isolated": true}, "expected_output": {"isolated": true}, "weight": 5},
      {"name": "Método no soportado lanza error", "input": {"method": "unknown"}, "expected_output": {"error": true}, "weight": 2}
    ]'::jsonb,
    '[
      {"type": "complexity-max", "config": {"max": 5}},
      {"type": "no-any-type", "config": {"severity": "error"}}
    ]'::jsonb,
    $code$interface PaymentMethod {
  process(order: Order, data: PaymentData): Promise<boolean>;
}

class CreditCardPayment implements PaymentMethod {
  async process(order: Order, data: PaymentData): Promise<boolean> {
    const validated = validateCard(data.cardNumber, data.cvv, data.expiry);
    if (!validated) throw new Error('Invalid card');
    const charged = await chargeCard(data.cardNumber!, order.total);
    return charged;
  }
}

class PaypalPayment implements PaymentMethod {
  async process(order: Order, data: PaymentData): Promise<boolean> {
    const token = await getPaypalToken(data.code!);
    return await executePaypal(token, order.total);
  }
}

class PaymentService {
  private methods: Map<string, PaymentMethod> = new Map();

  register(name: string, method: PaymentMethod): void {
    this.methods.set(name, method);
  }

  async process(order: Order, methodName: string, data: PaymentData): Promise<boolean> {
    const method = this.methods.get(methodName);
    if (!method) throw new Error('Unsupported payment method');
    const success = await method.process(order, data);
    if (success) await this.onSuccess(order);
    else await this.onFailure(order);
    return success;
  }

  private async onSuccess(order: Order): Promise<void> {
    await sendEmail(order.userId, 'payment-ok');
    await updateInventory(order.items);
  }

  private async onFailure(order: Order): Promise<void> {
    await notifyFailure(order.userId);
  }
}$code$,
    'Se aplica el patrón Strategy: cada tipo de pago es una clase que implementa PaymentMethod. PaymentService registra los métodos y delega. El post-procesamiento común (email, inventario) se centraliza. Cada handler es testeable de forma aislada.',
    150, 60, 0, true, 20, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 4. ch-callback — Callback Hell (TypeScript, refactor, mid)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-callback',
    'Registro de Usuario Encadenado',
    'Esta función registra un usuario nuevo encadenando varias operaciones asíncronas con callbacks. Funciona cuando todo va bien, pero cualquier error intermedio complica el debugging y el código es difícil de seguir. Investiga el código y encuentra los problemas de mantenibilidad y robustez.',
    'mid', 'refactor', 'typescript', 'https://github.com/example/user-service',
    $code$function createUser(email: string, password: string, callback: (err?: Error) => void) {
  validateEmail(email, (err) => {
    if (err) return callback(err);
    hashPassword(password, (err, hash) => {
      if (err) return callback(err);
      db.query('INSERT INTO users (email, password) VALUES ($1, $2)', [email, hash], (err) => {
        if (err) return callback(err);
        sendWelcomeEmail(email, (err) => {
          if (err) return callback(err);
          logAudit('user-created', email, () => {
            callback();
          });
        });
      });
    });
  });
}$code$,
    'Callback Hell',
    'available', '2025-01-04T00:00:00Z',
    '["Convertir callbacks anidados en async/await", "Manejar errores asíncronos con try/catch"]'::jsonb,
    '["Convertir a promesas sin async/await (solo añade .then)", "Olvidar propagar errores en el callback final", "Mezclar callbacks y async/await en la misma función"]'::jsonb,
    '[
      {"level": 1, "content": "Cuenta cuantos niveles de anidamiento hay. ¿Qué pasa si quieres agregar un sexto paso?", "cost_points": 0},
      {"level": 2, "content": "El lenguaje moderno tiene una sintaxis para escribir código asíncrono de forma lineal. ¿Cuál es?", "cost_points": 10},
      {"level": 3, "content": "La solución es convertir las funciones a promesas (util.promisify o async/await) y reescribir con try/catch.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "maintainability", "message": "El anidamiento de callbacks hace el código ilegible y difícil de mantener", "evidence": "validateEmail(email, (err) => { hashPassword(password, (err, hash) => { db.query(...) })", "suggested_fix": "Convertir a async/await con try/catch, o al menos a promesas encadenadas con .then()"},
      {"severity": "medium", "category": "error-handling", "message": "El manejo de errores se repite en cada nivel sin diferenciación", "evidence": "if (err) return callback(err);", "suggested_fix": "Centralizar el manejo de errores en un catch único, permitiendo distinguir tipos de error"},
      {"severity": "low", "category": "maintainability", "message": "La firma usa callbacks manuales en lugar de la convención del ecosistema", "evidence": "callback: (err?: Error) => void", "suggested_fix": "Devolver una Promise<void> para integrarse con async/await"}
    ]'::jsonb,
    '[
      {"name": "Registro exitoso completa todos los pasos", "input": {"email": "test@example.com", "password": "123456"}, "expected_output": {"success": true}, "weight": 5},
      {"name": "Email inválido lanza error", "input": {"email": "not-an-email", "password": "123456"}, "expected_output": {"error": "invalid_email"}, "weight": 3},
      {"name": "No hay mas de 2 niveles de anidamiento", "input": {"check_nesting": true}, "expected_output": {"max_depth": 2}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "max-nested-callbacks", "config": {"max": 2}},
      {"type": "prefer-async-await", "config": {"severity": "warn"}}
    ]'::jsonb,
    $code$async function createUser(email: string, password: string): Promise<void> {
  if (!await validateEmail(email)) {
    throw new Error('Invalid email');
  }

  const hash = await hashPassword(password);

  try {
    await db.query('INSERT INTO users (email, password) VALUES ($1, $2)', [email, hash]);
  } catch (err) {
    throw new Error(`Failed to create user: ${(err as Error).message}`);
  }

  await sendWelcomeEmail(email);
  await logAudit('user-created', email);
}$code$,
    'Se convierten los callbacks a async/await. Cada paso se ejecuta linealmente con await. El error handling se centraliza en un try/catch para la inserción en DB. La función devuelve Promise<void> en lugar de recibir un callback manual.',
    150, 50, 0, true, 15, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 5. ch-mutation — Prop Mutation (TypeScript, style, junior)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-mutation',
    'Lista de Usuarios con Eliminación',
    'Este componente recibe una lista de usuarios y permite eliminarlos. El padre pasa el array y el hijo lo modifica directamente. Parece funcionar, pero a veces provoca efectos secundarios en el estado del padre. Examina el código y encuentra los problemas.',
    'junior', 'style', 'typescript', 'https://github.com/example/angular-app',
    $code$import { Component, Input } from '@angular/core';

@Component({
  selector: 'user-list',
  template: `
    <div *ngFor="let user of users">
      {{ user.name }}
      <button (click)="deleteUser(user)">Delete</button>
    </div>
  `
})
export class UserListComponent {
  @Input() users: User[] = [];

  deleteUser(target: User) {
    const index = this.users.indexOf(target);
    if (index > -1) {
      this.users.splice(index, 1);
    }
  }
}$code$,
    'Prop Mutation',
    'available', '2025-01-05T00:00:00Z',
    '["Entender el flujo de datos unidireccional en Angular", "Emitir eventos al padre en lugar de mutar el input"]'::jsonb,
    '["Clonar el array y mutar el clon", "Usar readonly pero mutar igual", "Pensar que splice no muta porque devuelve el resultado"]'::jsonb,
    '[
      {"level": 1, "content": "Mira el metodo deleteUser. Modifica algo que vino de afuera del componente?", "cost_points": 0},
      {"level": 2, "content": "En Angular, los componentes hijos comunican cambios al padre mediante un mecanismo especifico. ¿Cual es?", "cost_points": 10},
      {"level": 3, "content": "La solucion es usar @Output EventEmitter para emitir el usuario a eliminar y que el padre actualice su propio array.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "style", "message": "El componente hijo muta directamente el @Input recibido del padre", "evidence": "this.users.splice(index, 1)", "suggested_fix": "Emitir un evento @Output con el usuario a eliminar y que el padre gestione la mutacion de su estado"},
      {"severity": "medium", "category": "maintainability", "message": "La plantilla usa directiva estructural deprecada en lugar del nuevo control flow", "evidence": "*ngFor=\"let user of users\"", "suggested_fix": "Migrar a @for (user of users; track user.id) { }"}
    ]'::jsonb,
    '[
      {"name": "Eliminar emite evento al padre", "input": {"action": "delete", "user_id": "u-1"}, "expected_output": {"emitted": true, "mutated_input": false}, "weight": 5},
      {"name": "El array del padre no cambia directamente", "input": {"check_mutation": true}, "expected_output": {"input_mutated": false}, "weight": 5},
      {"name": "Usuario eliminado deja la lista consistente", "input": {"users_count": 3, "delete_index": 1}, "expected_output": {"remaining": 2}, "weight": 3}
    ]'::jsonb,
    '[
      {"type": "no-input-mutation", "config": {"severity": "error"}},
      {"type": "angular-new-control-flow", "config": {"enforce": true}}
    ]'::jsonb,
    $code$import { Component, Input, Output, EventEmitter } from '@angular/core';

@Component({
  selector: 'user-list',
  template: `
    @for (user of users; track user.id) {
      <div>
        {{ user.name }}
        <button (click)="deleteUser(user)">Delete</button>
      </div>
    }
  `
})
export class UserListComponent {
  @Input({ required: true }) users: readonly User[] = [];
  @Output() userDeleted = new EventEmitter<User>();

  deleteUser(target: User): void {
    this.userDeleted.emit(target);
  }
}$code$,
    'El componente ya no muta el @Input. En su lugar, emite un evento @Output con el usuario a eliminar. El padre recibe el evento y actualiza su propio array. El input se marca como readonly para prevenir mutacion accidental. El template migra a @for (nuevo control flow).',
    100, 40, 0, false, 10, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 6. ch-dead — Dead Code (TypeScript, refactor, junior)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-dead',
    'Cálculo de Descuentos',
    'Esta función calcula descuentos según el tipo de cliente. Funciona, pero contiene caminos que nunca se recorren y variables que no se usan. Dedica tiempo a leer todo el código con cuidado y encuentra los problemas.',
    'junior', 'refactor', 'typescript', 'https://github.com/example/legacy-codebase',
    $code$function calculateDiscount(price: number, type: string): number {
  let discount = 0;
  const TAX_RATE = 0.21;
  const result = price * TAX_RATE;

  if (type === 'none') return 0;
  if (type === 'seasonal') {
    discount = price * 0.2;
  } else if (type === 'loyalty') {
    discount = price * 0.15;
  } else if (type === 'clearance') {
    discount = price * 0.5;
  } else if (type === 'vip') {
    discount = price * 0.3;
  } else if (type === 'employee') {
    discount = price * 0.4;
  } else {
    discount = price * 0.1;
  }

  if (price > 1000) {
    const extra = 50;
    discount += extra;
    return discount;
  }

  if (discount > price * 0.8) {
    return price * 0.8;
  }

  return discount;
}$code$,
    'Dead Code',
    'available', '2025-01-06T00:00:00Z',
    '["Identificar variables y ramas inalcanzables", "Eliminar código muerto sin miedo"]'::jsonb,
    '["Dejar el código muerto con un comentario por si acaso", "Renombrar variables muertas en lugar de eliminarlas", "No cubrir el caso default con un tipo discriminated union"]'::jsonb,
    '[
      {"level": 1, "content": "Hay variables declaradas que no se usan en ningún lado. ¿Puedes encontrarlas?", "cost_points": 0},
      {"level": 2, "content": "Hay un bloque condicional que nunca se ejecuta por un return anterior. ¿Lo ves?", "cost_points": 10},
      {"level": 3, "content": "La solucion es eliminar las variables muertas (TAX_RATE, result) y el bloque inalcanzable del cap de descuento.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "medium", "category": "maintainability", "message": "Hay variables declaradas que nunca se usan", "evidence": "const TAX_RATE = 0.21; const result = price * TAX_RATE;", "suggested_fix": "Eliminar TAX_RATE y result"},
      {"severity": "medium", "category": "logic", "message": "Hay un bloque condicional que nunca se ejecuta por un return previo", "evidence": "if (price > 1000) { return discount; } if (discount > price * 0.8) { ... }", "suggested_fix": "Mover el cap de descuento antes del return o eliminarlo si no es necesario"},
      {"severity": "low", "category": "maintainability", "message": "El cálculo de descuentos usa if/else en cadena en lugar de un mapa", "evidence": "if (type === ''seasonal'') { ... } else if (type === ''loyalty'') { ... }", "suggested_fix": "Usar un record/mapa de tipos a porcentajes"}
    ]'::jsonb,
    '[
      {"name": "Descuento seasonal correcto", "input": {"price": 500, "type": "seasonal"}, "expected_output": {"discount": 100}, "weight": 3},
      {"name": "Precio alto con extra", "input": {"price": 1500, "type": "loyalty"}, "expected_output": {"discount": 275}, "weight": 3},
      {"name": "Tipo none devuelve 0", "input": {"price": 100, "type": "none"}, "expected_output": {"discount": 0}, "weight": 2},
      {"name": "No hay variables sin usar", "input": {"check_unused": true}, "expected_output": {"unused_count": 0}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "no-unused-vars", "config": {"severity": "error"}},
      {"type": "no-unreachable", "config": {"severity": "error"}}
    ]'::jsonb,
    $code$const DISCOUNT_RATES: Record<string, number> = {
  none: 0,
  seasonal: 0.2,
  loyalty: 0.15,
  clearance: 0.5,
  vip: 0.3,
  employee: 0.4,
};

const HIGH_VALUE_THRESHOLD = 1000;
const HIGH_VALUE_BONUS = 50;
const MAX_DISCOUNT_RATIO = 0.8;

function calculateDiscount(price: number, type: string): number {
  const rate = DISCOUNT_RATES[type] ?? 0.1;
  let discount = price * rate;

  if (price > HIGH_VALUE_THRESHOLD) {
    discount += HIGH_VALUE_BONUS;
  }

  return Math.min(discount, price * MAX_DISCOUNT_RATIO);
}$code$,
    'Se eliminan las variables muertas (TAX_RATE, result) y el bloque inalcanzable. Los porcentajes se mueven a un mapa (Record<string, number>) eliminando la cadena de if/else. Las constantes se nombran de forma descriptiva. El cap de descuento se aplica con Math.min de forma que siempre se ejecuta.',
    100, 40, 0, false, 10, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 7. ch-errors — Silent Failures (TypeScript, refactor, mid)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-errors',
    'Carga de Datos del Dashboard',
    'Este componente carga datos del dashboard al inicializarse. Funciona cuando la API responde bien, pero si hay un error de red, el usuario ve una pantalla en blanco sin explicación. Investiga el código y encuentra los problemas de robustez.',
    'mid', 'refactor', 'typescript', 'https://github.com/example/dashboard-app',
    $code$import { Component, OnInit } from '@angular/core';

@Component({
  selector: 'app-dashboard',
  template: `
    <h1>Welcome {{ user.name }}</h1>
    <div *ngFor="let item of items">{{ item.name }}</div>
  `
})
export class DashboardComponent implements OnInit {
  user: any;
  items: any[] = [];
  dashboardStats: any;

  async ngOnInit() {
    const userResp = await fetch('/api/user');
    this.user = await userResp.json();

    const itemsResp = await fetch('/api/items');
    this.items = await itemsResp.json();

    const stats = await fetch('/api/stats');
    this.dashboardStats = await stats.json();
  }
}$code$,
    'Silent Failures',
    'available', '2025-01-07T00:00:00Z',
    '["Manejar errores de red de forma visible para el usuario", "Tipar respuestas de API en lugar de usar any"]'::jsonb,
    '["Usar try/catch pero mostrar el error en un console.log", "Agregar un loading: boolean sin manejar el error", "Tipar las respuestas pero no validar el status HTTP"]'::jsonb,
    '[
      {"level": 1, "content": "Que pasa si fetch falla? Hay algun manejo de error?", "cost_points": 0},
      {"level": 2, "content": "Si la API devuelve 500, fetch no lanza. Como sabrias que fallo sin revisar response.ok?", "cost_points": 10},
      {"level": 3, "content": "La solucion es envolver cada fetch en try/catch, verificar response.ok, y mostrar estados de error/loading en el template.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "error-handling", "message": "No hay manejo de errores: si fetch falla, la Promise rechazada no se captura", "evidence": "const userResp = await fetch(''/api/user''); this.user = await userResp.json();", "suggested_fix": "Envolver en try/catch y mostrar un estado de error al usuario"},
      {"severity": "medium", "category": "error-handling", "message": "No se verifica response.ok antes de parsear el JSON", "evidence": "this.user = await userResp.json();", "suggested_fix": "Verificar response.ok y lanzar si es false antes de hacer .json()"},
      {"severity": "medium", "category": "maintainability", "message": "Todos los tipos son any, sin validacion ni tipado de las respuestas", "evidence": "user: any; items: any[] = []; dashboardStats: any;", "suggested_fix": "Definir interfaces User, Item, DashboardStats y tipar las variables"},
      {"severity": "low", "category": "maintainability", "message": "El template usa directiva estructural deprecada", "evidence": "*ngFor=\"let item of items\"", "suggested_fix": "Migrar a @for (item of items; track item.id) { }"}
    ]'::jsonb,
    '[
      {"name": "Error de red muestra mensaje al usuario", "input": {"api_status": 500}, "expected_output": {"error_shown": true}, "weight": 5},
      {"name": "Carga exitosa muestra datos", "input": {"api_status": 200}, "expected_output": {"error_shown": false, "data_loaded": true}, "weight": 3},
      {"name": "Carga fallida no deja la app en estado inconsistente", "input": {"api_partial_fail": true}, "expected_output": {"consistent_state": true}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "no-floating-promises", "config": {"severity": "error"}},
      {"type": "no-explicit-any", "config": {"severity": "error"}}
    ]'::jsonb,
    $code$import { Component, OnInit, signal } from '@angular/core';

interface User { name: string; }
interface Item { id: string; name: string; }
interface DashboardStats { total: number; }

@Component({
  selector: 'app-dashboard',
  template: `
    @if (loading()) {
      <p>Loading...</p>
    }
    @if (error()) {
      <p class="error">{{ error() }}</p>
    }
    @if (user()) {
      <h1>Welcome {{ user()!.name }}</h1>
      @for (item of items(); track item.id) {
        <div>{{ item.name }}</div>
      }
    }
  `
})
export class DashboardComponent implements OnInit {
  user = signal<User | null>(null);
  items = signal<Item[]>([]);
  dashboardStats = signal<DashboardStats | null>(null);
  loading = signal(false);
  error = signal<string | null>(null);

  async ngOnInit(): Promise<void> {
    this.loading.set(true);
    this.error.set(null);

    try {
      const [user, items, stats] = await Promise.all([
        this.fetchApi<User>('/api/user'),
        this.fetchApi<Item[]>('/api/items'),
        this.fetchApi<DashboardStats>('/api/stats'),
      ]);

      this.user.set(user);
      this.items.set(items);
      this.dashboardStats.set(stats);
    } catch (err) {
      this.error.set(`Failed to load dashboard: ${(err as Error).message}`);
    } finally {
      this.loading.set(false);
    }
  }

  private async fetchApi<T>(url: string): Promise<T> {
    const resp = await fetch(url);
    if (!resp.ok) {
      throw new Error(`API error: ${resp.status} ${resp.statusText}`);
    }
    return resp.json() as Promise<T>;
  }
}$code$,
    'Se agrega try/catch alrededor de los fetches. Se verifica response.ok antes de parsear JSON. Se usan signals con estado tipado (User, Item, DashboardStats). Se muestra loading y error en el template con el nuevo control flow (@if). Las tres llamadas se paralelizan con Promise.all.',
    150, 50, 0, true, 15, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;

-- =============================================================================
-- 8. ch-naming — Poor Naming (TypeScript, style, junior)
-- =============================================================================
INSERT INTO public.challenges (
    id, title, description, difficulty, category, language, repo_url, code, code_smell, status, created_at,
    learning_objectives, common_mistakes, hints, expected_findings, test_cases, linter_rules,
    solution_code, solution_explanation,
    base_points, bonus_points, penalty_per_hint, time_bonus, estimated_time_minutes, origin, created_by
) VALUES (
    'ch-naming',
    'Cálculo Financiero Opaco',
    'Esta función realiza cálculos financieros. El resultado es correcto, pero para entender qué hace necesitas ingeniería inversa. Cada variable te obliga a inferir su propósito. Identifica los problemas de legibilidad.',
    'junior', 'style', 'typescript', 'https://github.com/example/obfuscated-code',
    $code$function calc(a: number, b: number, c: string): number {
  const d = new Date();
  const e = d.getFullYear() - a;
  const f = e * 12 + b;

  let g = 0;
  for (let i = 0; i < f; i++) {
    const h = c === 'monthly' ? 1 : c === 'yearly' ? 12 : 0;
    const j = f - i;
    const k = 0.05 / 12;
    const l = Math.pow(1 + k, j);
    const m = k * l / (l - 1);
    g += h * m;
  }

  const n = g * a / 100;
  return Math.round(n * 100) / 100;
}$code$,
    'Poor Naming',
    'available', '2025-01-08T00:00:00Z',
    '["Reemplazar nombres de una letra por nombres descriptivos", "Reemplazar ternarios anidados por estructuras legibles"]'::jsonb,
    '["Renombrar pero dejar comentarios que explican lo que ya dice el nombre", "Usar nombres largos sin contexto (theVariableThatCalculates)", "Añadir tipos union sin enums para c"]'::jsonb,
    '[
      {"level": 1, "content": "Toma la variable f. Simplifica su expresion: que representa? Si a es año de nacimiento y b son meses extra...", "cost_points": 0},
      {"level": 2, "content": "La variable c parece ser un tipo de frecuencia (monthly o yearly). Como lo expresarias de forma clara?", "cost_points": 10},
      {"level": 3, "content": "La solucion es renombrar a: birthYear, monthsSinceBirth, frequency, interestRate, totalPayment. Extraer una funcion para el pago mensual.", "cost_points": 25}
    ]'::jsonb,
    '[
      {"severity": "high", "category": "readability", "message": "Todos los nombres son de una letra, sin contexto ni significado", "evidence": "const d = new Date(); const e = d.getFullYear() - a; const f = e * 12 + b;", "suggested_fix": "Renombrar con nombres descriptivos: birthYear, ageInMonths, totalMonths"},
      {"severity": "medium", "category": "readability", "message": "Parametro c usa strings magicos sin tipo union ni enum", "evidence": "c === ''monthly'' ? 1 : c === ''yearly'' ? 12 : 0", "suggested_fix": "Definir type Frequency = ''monthly'' | ''yearly'' y extraer la conversion a una funcion"},
      {"severity": "low", "category": "readability", "message": "Constantes magicas sin nombre (0.05, 100)", "evidence": "const k = 0.05 / 12; const n = g * a / 100;", "suggested_fix": "Nombrar constantes: INTEREST_RATE, LOAN_AMOUNT_RATIO"}
    ]'::jsonb,
    '[
      {"name": "Calculo mensual da el mismo resultado", "input": {"birthYear": 1990, "extraMonths": 2, "frequency": "monthly"}, "expected_output": {"matches_original": true}, "weight": 5},
      {"name": "Calculo anual da el mismo resultado", "input": {"birthYear": 1980, "extraMonths": 0, "frequency": "yearly"}, "expected_output": {"matches_original": true}, "weight": 3},
      {"name": "No hay variables de una letra excepto index de loop", "input": {"check_naming": true}, "expected_output": {"single_letter_vars": 1}, "weight": 5}
    ]'::jsonb,
    '[
      {"type": "no-single-letter-var", "config": {"allow_loop_index": true}},
      {"type": "no-magic-numbers", "config": {"severity": "warn"}}
    ]'::jsonb,
    $code$type Frequency = 'monthly' | 'yearly';

const INTEREST_RATE = 0.05;
const LOAN_AMOUNT_RATIO = 100;
const MONTHS_PER_YEAR = 12;

function frequencyToMonths(frequency: Frequency): number {
  return frequency === 'monthly' ? 1 : MONTHS_PER_YEAR;
}

function calculateMonthlyPayment(
  birthYear: number,
  extraMonths: number,
  frequency: Frequency,
): number {
  const currentYear = new Date().getFullYear();
  const ageInYears = currentYear - birthYear;
  const totalMonths = ageInYears * MONTHS_PER_YEAR + extraMonths;

  let totalPayment = 0;
  for (let i = 0; i < totalMonths; i++) {
    const monthsPerPeriod = frequencyToMonths(frequency);
    const remainingMonths = totalMonths - i;
    const monthlyRate = INTEREST_RATE / MONTHS_PER_YEAR;
    const compoundFactor = Math.pow(1 + monthlyRate, remainingMonths);
    const monthlyFactor = (monthlyRate * compoundFactor) / (compoundFactor - 1);
    totalPayment += monthsPerPeriod * monthlyFactor;
  }

  const result = (totalPayment * birthYear) / LOAN_AMOUNT_RATIO;
  return Math.round(result * 100) / 100;
}$code$,
    'Todas las variables se renombran de forma descriptiva: birthYear, extraMonths, frequency. El tipo Frequency reemplaza strings magicos. Las constantes se nombran (INTEREST_RATE, MONTHS_PER_YEAR). La conversion de frecuencia se extrae a una funcion pura. Solo el index del loop queda como variable de una letra.',
    100, 40, 0, false, 10, 'curated', 'system'
)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    code = EXCLUDED.code,
    learning_objectives = EXCLUDED.learning_objectives,
    common_mistakes = EXCLUDED.common_mistakes,
    hints = EXCLUDED.hints,
    expected_findings = EXCLUDED.expected_findings,
    test_cases = EXCLUDED.test_cases,
    linter_rules = EXCLUDED.linter_rules,
    solution_code = EXCLUDED.solution_code,
    solution_explanation = EXCLUDED.solution_explanation,
    base_points = EXCLUDED.base_points,
    bonus_points = EXCLUDED.bonus_points,
    penalty_per_hint = EXCLUDED.penalty_per_hint,
    time_bonus = EXCLUDED.time_bonus,
    estimated_time_minutes = EXCLUDED.estimated_time_minutes,
    origin = EXCLUDED.origin,
    created_by = EXCLUDED.created_by;