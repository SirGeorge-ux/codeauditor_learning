// MockChallengeRepository — infrastructure adapter with 8 realistic vulnerable challenges.
//
// Each challenge contains real TypeScript code demonstrating the code smell,
// not placeholder text. Challenges are designed for the audit dojo experience.
import { Challenge } from "../../domain/models/challenge";
import { ChallengeRepository } from "../../domain/ports/challenge-repository.port";

const CHALLENGES: Challenge[] = [
  {
    id: "ch-sqli",
    difficulty: "junior",
    category: "security",
    language: "typescript",
    title: "SQL Injection en Login",
    codeSmell: "SQL Injection",
    description:
      "Este endpoint de login es vulnerable a SQL Injection. La entrada del usuario se concatena directamente en la consulta SQL sin sanitizar. Un atacante puede manipular el parámetro username para ejecutar comandos SQL arbitrarios.",
    repoUrl: "https://github.com/example/vulnerable-api",
    code: `import { db } from './database';

export async function login(username: string, password: string) {
  const query = \`SELECT * FROM users WHERE username = '\${username}' AND password = '\${password}'\`;
  const user = await db.execute(query);

  if (user.rows.length > 0) {
    return { success: true, token: generateToken(user.rows[0]) };
  }

  return { success: false };
}`,
    status: "available",
    createdAt: new Date("2025-01-01"),
  },
  {
    id: "ch-xss",
    difficulty: "junior",
    category: "security",
    language: "typescript",
    title: "XSS en Comentarios",
    codeSmell: "Cross-Site Scripting",
    description:
      "Este componente renderiza comentarios de usuarios directamente con innerHTML, sin sanitizar. Un usuario malicioso puede inyectar scripts arbitrarios que se ejecutarán en el navegador de otros usuarios.",
    repoUrl: "https://github.com/example/social-app",
    code: `import { Component, Input } from '@angular/core';

@Component({
  selector: 'comment-thread',
  template: '<div [innerHTML]="renderComment()"></div>'
})
export class CommentThread {
  @Input() comments: Comment[] = [];

  renderComment(): string {
    return this.comments
      .map(c => \`<div class="comment">\${c.body}</div>\`)
      .join('');
  }
}`,
    status: "available",
    createdAt: new Date("2025-01-02"),
  },
  {
    id: "ch-god",
    difficulty: "mid",
    category: "design",
    language: "typescript",
    title: "Diosidad / Cyclomatic Complexity",
    codeSmell: "God Function",
    description:
      "Esta función handlePayment procesa TODOS los tipos de pago en un solo bloque. Tiene complejidad ciclomática altísima, mezcla responsabilidades y es imposible de testear por separado. Cualquier cambio en un método de pago puede romper los otros.",
    repoUrl: "https://github.com/example/payment-service",
    code: `async function handlePayment(order: Order, method: string, data: any) {
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
}`,
    status: "available",
    createdAt: new Date("2025-01-03"),
  },
  {
    id: "ch-callback",
    difficulty: "junior",
    category: "async",
    language: "typescript",
    title: "Callback Hell / Promesas Anidadas",
    codeSmell: "Callback Hell",
    description:
      "Esta función procesa un usuario nuevo con una cadena de callbacks anidados. Es ilegible, difícil de debuggear, y cualquier error en un paso intermedio deja el sistema en un estado inconsistente porque no hay manejo de errores.",
    repoUrl: "https://github.com/example/user-service",
    code: `function createUser(email: string, password: string, callback: (err?: Error) => void) {
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
}`,
    status: "available",
    createdAt: new Date("2025-01-04"),
  },
  {
    id: "ch-mutation",
    difficulty: "junior",
    category: "angular",
    language: "typescript",
    title: "Mutación de Props en Componentes",
    codeSmell: "Prop Mutation",
    description:
      "Este componente hijo muta directamente la propiedad recibida del padre. En Angular, los @Input no deben modificarse internamente. Esto causa efectos secundarios impredecibles y bugs intermitentes difíciles de rastrear.",
    repoUrl: "https://github.com/example/angular-app",
    code: `import { Component, Input } from '@angular/core';

@Component({
  selector: 'user-list',
  template: \`
    <div *ngFor="let user of users">
      {{ user.name }}
      <button (click)="deleteUser(user)">Delete</button>
    </div>
  \`
})
export class UserListComponent {
  @Input() users: User[] = [];

  deleteUser(target: User) {
    const index = this.users.indexOf(target);
    if (index > -1) {
      this.users.splice(index, 1); // MUTATING THE INPUT!
    }
  }
}`,
    status: "available",
    createdAt: new Date("2025-01-05"),
  },
  {
    id: "ch-dead",
    difficulty: "junior",
    category: "logic",
    language: "typescript",
    title: "Código Muerto / Condiciones Redundantes",
    codeSmell: "Dead Code",
    description:
      "Esta función está llena de condiciones que nunca se cumplen, branches inalcanzables, y variables que se asignan pero nunca se usan. El código muerto aumenta la carga cognitiva y sugiere que hubo refactors incompletos.",
    repoUrl: "https://github.com/example/legacy-codebase",
    code: `function calculateDiscount(price: number, type: string): number {
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

  // This never executes if price > 1000
  if (discount > price * 0.8) {
    return price * 0.8;
  }

  return discount;
}`,
    status: "available",
    createdAt: new Date("2025-01-06"),
  },
  {
    id: "ch-errors",
    difficulty: "junior",
    category: "error-handling",
    language: "typescript",
    title: "Falta de Manejo de Errores",
    codeSmell: "Silent Failures",
    description:
      "Esta función asume que todas las operaciones asíncronas van a funcionar perfectamente. No hay try/catch, no hay validación de respuestas, no hay manejo de errores de red. Si la API falla, el usuario ve una pantalla en blanco sin explicación.",
    repoUrl: "https://github.com/example/dashboard-app",
    code: `import { Component, OnInit } from '@angular/core';

@Component({
  selector: 'app-dashboard',
  template: \`
    <h1>Welcome {{ user.name }}</h1>
    <div *ngFor="let item of items">{{ item.name }}</div>
  \`
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
}`,
    status: "available",
    createdAt: new Date("2025-01-07"),
  },
  {
    id: "ch-naming",
    difficulty: "junior",
    category: "readability",
    language: "typescript",
    title: "Variables con Naming Opaco",
    codeSmell: "Poor Naming",
    description:
      "Esta función usa nombres de variables crípticos que obligan al lector a hacer ingeniería inversa para entender qué hace cada cosa. En un código profesional, el nombre de una variable debe revelar su intención sin necesidad de comentarios.",
    repoUrl: "https://github.com/example/obfuscated-code",
    code: `function calc(a: number, b: number, c: string): number {
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
}`,
    status: "available",
    createdAt: new Date("2025-01-08"),
  },
];

export class MockChallengeRepository implements ChallengeRepository {
  async getAll(): Promise<Challenge[]> {
    return CHALLENGES;
  }

  async getById(id: string): Promise<Challenge | null> {
    return CHALLENGES.find((c) => c.id === id) ?? null;
  }
}