// tutor-chat-message.component.ts — dumb component that renders a
// single ChatMessage. The component is presentation-only: it
// takes a `message` input and emits nothing. Markdown rendering
// is intentionally simple (paragraph + code-block detection) to
// avoid pulling in a heavy library; the spec test pins the
// rendering contract so a future refactor doesn't silently drop
// code-block syntax highlighting.
import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { CommonModule } from '@angular/common';

import { ChatMessage } from '../../../domain/models/chat';

@Component({
  selector: 'app-tutor-chat-message',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="flex w-full"
      [class.justify-end]="message().role === 'user'"
      [class.justify-start]="message().role !== 'user'"
    >
      <div
        class="max-w-[85%] rounded-sm px-3 py-2 text-sm leading-relaxed border"
        [class.bg-dojo-surface]="message().role === 'user'"
        [class.text-dojo-text]="true"
        [class.border-dojo-border]="true"
        [class.bg-dojo-base]="message().role === 'assistant'"
        [class.bg-amber-950]="message().role === 'system'"
        [class.text-amber-200]="message().role === 'system'"
      >
        @if (message().role === 'assistant' && message().socraticLevel !== undefined) {
          <div class="text-xs text-dojo-text opacity-60 mb-1">
            nivel socrático {{ message().socraticLevel }}
          </div>
        }

        @if (parts().length === 1 && !parts()[0].code) {
          <p class="whitespace-pre-wrap break-words">{{ parts()[0].text }}</p>
        } @else {
          @for (part of parts(); track $index) {
            @if (part.code) {
              <pre
                class="my-2 px-2 py-1 rounded-sm bg-dojo-base border border-dojo-border overflow-x-auto text-xs"
              ><code>{{ part.text }}</code></pre>
            } @else {
              <p class="whitespace-pre-wrap break-words">{{ part.text }}</p>
            }
          }
        }

        <div class="text-[10px] text-dojo-text opacity-50 mt-1">
          {{ message().timestamp | date: 'HH:mm' }}
        </div>
      </div>
    </div>
  `,
})
export class TutorChatMessageComponent {
  // input.required is the Angular 21.2 signal-based input API
  // (the project convention after chore/fix-frontend-debt).
  readonly message = input.required<ChatMessage>();

  // parts splits the content into a sequence of (text, code)
  // tuples so the template can render inline code blocks with
  // monospace font + dark background. The split is naive
  // (fence-style ``` blocks only) but enough for the v1 UX.
  // The first line of a code block is the language tag (e.g.
  // "ts") — we strip it so the rendered code doesn't include
  // the fence label.
  readonly parts = computed<{ text: string; code: boolean }[]>(() => {
    const content = this.message().content ?? '';
    if (content.length === 0) {
      return [{ text: '', code: false }];
    }
    const segments: { text: string; code: boolean }[] = [];
    const regex = /```([\s\S]*?)```/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = regex.exec(content)) !== null) {
      if (match.index > lastIndex) {
        segments.push({ text: content.slice(lastIndex, match.index), code: false });
      }
      // Strip the optional language tag from the first line of
      // the code block (markdown convention: ```ts\ncode\n```).
      const body = match[1].replace(/^[a-zA-Z0-9_-]+\n/, '');
      segments.push({ text: body, code: true });
      lastIndex = regex.lastIndex;
    }
    if (lastIndex < content.length) {
      segments.push({ text: content.slice(lastIndex), code: false });
    }
    return segments.length > 0 ? segments : [{ text: content, code: false }];
  });
}
