// practice-page.component.ts — minimal stub for the practice mode
// page. The full S3 implementation is scaffolded in
// openspec/changes/free-practice-mode/; this stub exists so PR 3
// (tutor-chat) can land cleanly with the chat panel wired into
// every route that design.md §6 names (dojo + practice).
//
// On desktop the chat panel mounts inline; on mobile (<768px) a
// FAB toggle swaps the panel into a modal. For the stub we
// render the inline panel only — the FAB modal can land in the
// free-practice-mode change when the rest of the practice page
// is built.
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';

import { TutorChatPanelComponent } from '../tutor-chat/tutor-chat-panel.component';

@Component({
  selector: 'app-practice-page',
  standalone: true,
  imports: [CommonModule, TutorChatPanelComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="h-full flex flex-col bg-dojo-base">
      <div
        class="h-10 flex items-center justify-between px-4 bg-dojo-surface border-b border-dojo-border"
      >
        <span class="text-xs text-dojo-text">Modo práctica libre</span>
        <span class="text-xs text-dojo-text opacity-60">Próximamente</span>
      </div>

      <div class="flex-1 flex overflow-hidden">
        <!-- Placeholder for the free-practice code panel (S3) -->
        <div class="flex-1 flex items-center justify-center">
          <div class="text-center text-dojo-text opacity-70 max-w-md px-6">
            <p class="text-lg font-semibold mb-2">Modo práctica libre</p>
            <p class="text-sm leading-relaxed">
              Próximamente: generación de ejercicios on-demand según el lenguaje y nivel
              del usuario. Mientras tanto, el tutor socrático está disponible
              <span class="text-dojo-accent">en el panel de la derecha</span>
              para responder preguntas sobre cualquier challenge.
            </p>
          </div>
        </div>

        <!-- Rightmost: Socratic Tutor Chat Panel -->
        <div class="w-80 flex-shrink-0">
          <app-tutor-chat-panel
            [routeContext]="{ route: '/practice' }"
            (onNavigate)="onTutorNavigate($event)"
          ></app-tutor-chat-panel>
        </div>
      </div>
    </div>
  `,
})
export class PracticePageComponent {
  private readonly router = inject(Router);

  onTutorNavigate(path: string): void {
    void this.router.navigateByUrl(path);
  }
}
