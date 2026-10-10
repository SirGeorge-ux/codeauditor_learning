// tutor-chat-panel.component.ts — smart panel that orchestrates
// the message list, the input, and the FAB/sidebar visibility
// state. The panel injects TutorChatUseCase and forwards events
// from the input child component to the use case.
//
// On viewports >= 768px the panel is mounted inline (right
// sidebar slot) by the parent page; on smaller viewports the
// page mounts a FAB button + modal that wraps the same
// TutorChatPanelComponent. The FAB/modal toggle is therefore
// the page's responsibility, not this component's. The panel
// only owns the `messages` rendering + the `sendMessage` /
// `handleCommand` dispatch.
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  HostBinding,
  computed,
  inject,
  input,
  output,
  viewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';

import { TutorChatMessageComponent } from './tutor-chat-message.component';
import { TutorChatInputComponent } from './tutor-chat-input.component';
import { TutorChatUseCase } from '../../../application/tutor-chat.use-case';

@Component({
  selector: 'app-tutor-chat-panel',
  standalone: true,
  imports: [CommonModule, TutorChatMessageComponent, TutorChatInputComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      data-testid="tutor-chat-panel"
      class="h-full w-full flex flex-col bg-dojo-surface border-l border-dojo-border"
    >
      <!-- Header -->
      <div class="flex items-center justify-between px-3 py-2 border-b border-dojo-border">
        <div class="flex items-center gap-2">
          <span class="text-dojo-accent text-sm">●</span>
          <span class="text-dojo-text text-sm font-semibold">Tutor</span>
          <span class="text-dojo-text opacity-60 text-xs">nivel {{ useCase.currentLevel() }}</span>
        </div>
        @if (useCase.lastOfferHint()) {
          <span class="text-amber-400 text-xs">¿Querés una pista?</span>
        }
      </div>

      <!-- Error banner (if any) -->
      @if (useCase.error(); as err) {
        <div class="px-3 py-1 bg-red-950 text-red-300 text-xs">{{ err }}</div>
      }

      <!-- Message list (scrollable) -->
      <div
        #messageList
        class="flex-1 overflow-y-auto p-3 flex flex-col gap-2"
        data-testid="tutor-chat-message-list"
      >
        @if (useCase.messages().length === 0) {
          <div class="text-dojo-text opacity-60 text-xs text-center my-4 leading-relaxed">
            <p>Hola. Soy tu tutor socrático.</p>
            <p class="mt-2">Nivel actual: {{ useCase.currentLevel() }} · Usá /help para ver los comandos.</p>
          </div>
        } @else {
          @for (msg of useCase.messages(); track msg.id) {
            <app-tutor-chat-message [message]="msg"></app-tutor-chat-message>
          }
        }
      </div>

      <!-- Input -->
      <app-tutor-chat-input
        [streaming]="useCase.streaming()"
        placeholder="Preguntale algo al tutor..."
        (send)="onSend($event)"
        (command)="onCommand($event)"
      ></app-tutor-chat-input>
    </div>
  `,
})
export class TutorChatPanelComponent {
  readonly useCase = inject(TutorChatUseCase);
  private readonly router = inject(Router);

  // routeContext is set by the parent page so the use case
  // can stamp the active Angular route on every message. The
  // page also passes challengeId + codeSnippet so the LLM has
  // the same context the user is looking at.
  readonly routeContext = input<{ route: string; challengeId?: string; codeSnippet?: string }>({
    route: '/',
  });

  // onNavigate emits the dictionary command's path so the
  // parent page can switch routes without the panel importing
  // @angular/router directly. The use case's navigation hook
  // fires onNavigate; the page handles the actual router call.
  readonly onNavigate = output<string>();

  // hidden allows the parent page to collapse the panel on
  // mobile without unmounting it (state survives).
  readonly hidden = input<boolean>(false);

  @HostBinding('class.hidden') get hostHidden(): boolean {
    return this.hidden();
  }

  private readonly messageListRef =
    viewChild<ElementRef<HTMLDivElement>>('messageList');

  constructor() {
    // Wire the navigation hook so /diccionario can route
    // through the page's onNavigate output (and then to
    // router.navigate) without circular dependencies.
    this.useCase.registerNavigateHook((path: string) => {
      this.onNavigate.emit(path);
    });
  }

  onSend(text: string): void {
    void this.useCase.sendMessage(text, this.routeContext());
  }

  onCommand(command: string): void {
    void this.useCase.handleCommand(command);
  }
}
