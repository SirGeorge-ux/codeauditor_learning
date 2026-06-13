import { Component } from "@angular/core";

/**
 * HomeComponent — the landing page component.
 *
 * Stub: displays a simple welcome message matching the Dojo dark theme.
 * Real implementation will include the audit form, recent sessions list, etc.
 */
@Component({
  selector: "app-home",
  standalone: true,
  template: `
    <div class="min-h-screen bg-dojo-base text-dojo-text flex items-center justify-center">
      <div class="text-center">
        <h1 class="text-4xl font-bold text-dojo-accent mb-4">CodeAuditor</h1>
        <p class="text-dojo-text text-lg">AI-powered code security auditing</p>
        <div class="mt-8 p-4 bg-dojo-surface rounded border border-dojo-border">
          <p class="text-dojo-text text-sm">Scaffolding ready — components to be implemented in next SDD changes.</p>
        </div>
      </div>
    </div>
  `,
  styles: [
    `
      :host {
        display: block;
      }
    `,
  ],
})
export class HomeComponent {}