// tutor-chat-input.component.ts — dumb component that owns the
// textarea + send button. Auto-resizes the textarea as the user
// types, dispatches (send) on Enter (without shift), and inserts
// a newline on Shift+Enter. Slash command detection: messages
// that start with "/" are dispatched to the use case via
// (command) instead of (send), so the parent can route them
// through handleCommand.
import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  input,
  output,
  viewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-tutor-chat-input',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <form
      class="flex flex-col gap-2 p-2 border-t border-dojo-border bg-dojo-surface"
      (submit)="onSubmit($event)"
    >
      <textarea
        #textarea
        data-testid="tutor-chat-input-textarea"
        rows="1"
        class="w-full px-2 py-1 rounded-sm bg-dojo-base border border-dojo-border text-dojo-text text-sm resize-none focus:outline-none focus:border-dojo-accent"
        [attr.aria-label]="placeholder()"
        [attr.placeholder]="placeholder()"
        [disabled]="streaming()"
        (keydown)="onKeydown($event)"
        (input)="onInput($event)"
      ></textarea>
      <div class="flex items-center justify-between text-xs">
        <span class="text-dojo-text opacity-60"
          >Enter para enviar · Shift+Enter para nueva línea</span
        >
        <button
          type="submit"
          data-testid="tutor-chat-input-send"
          class="px-3 py-1 rounded-sm bg-dojo-accent text-dojo-base font-medium disabled:opacity-50 disabled:cursor-not-allowed"
          [disabled]="streaming()"
        >
          Enviar
        </button>
      </div>
    </form>
  `,
})
export class TutorChatInputComponent {
  readonly streaming = input<boolean>(false);
  readonly placeholder = input<string>('Preguntale algo al tutor...');

  // Output events. messageSent fires for normal text messages;
  // commandSent fires for slash commands so the parent can route
  // them through the use case's command parser directly.
  // Naming avoids the @angular-eslint/no-output-native and
  // @angular-eslint/no-output-on-prefix rules.
  readonly messageSent = output<string>();
  readonly commandSent = output<string>();

  private readonly textareaRef = viewChild<ElementRef<HTMLTextAreaElement>>('textarea');

  onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      this.submit();
    }
  }

  onInput(_event: Event): void {
    const el = this.textareaRef()?.nativeElement;
    if (!el) return;
    // Auto-resize: reset to 1 row, then grow to content.
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }

  onSubmit(event: Event): void {
    event.preventDefault();
    this.submit();
  }

  private submit(): void {
    const el = this.textareaRef()?.nativeElement;
    if (!el) return;
    const value = el.value.trim();
    if (value.length === 0) return;
    el.value = '';
    el.style.height = 'auto';
    if (value.startsWith('/')) {
      this.commandSent.emit(value);
    } else {
      this.messageSent.emit(value);
    }
  }
}
