import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Challenge } from '../../../domain/models/challenge';

@Component({
  selector: 'app-context-panel',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="h-full w-full bg-dojo-surface rounded-sm overflow-y-auto p-4">
      @if (!challenge) {
        <div class="space-y-4">
          <!-- Challenge Description -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Challenge Description
            </h3>
            <p class="text-sm text-dojo-text" style="font-family: Inter, sans-serif;">
              No challenge selected. Select a challenge from the dashboard to begin auditing.
            </p>
          </div>

          <!-- Repository Origin -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Repository Origin
            </h3>
            <p class="text-xs text-dojo-text opacity-70" style="font-family: monospace;">
              —
            </p>
          </div>

          <!-- Code Smell Info -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Code Smell
            </h3>
            <div class="space-y-1">
              <span class="inline-block px-2 py-0.5 bg-dojo-surface rounded-sm text-xs text-dojo-text border border-dojo-border">
                None
              </span>
              <p class="text-xs text-dojo-text opacity-70" style="font-family: Inter, sans-serif;">
                No code smell selected.
              </p>
            </div>
          </div>
        </div>
      } @else {
        <div class="space-y-4">
          <!-- Challenge Title -->
          <div>
            <h3 class="text-lg font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              {{ challenge.title }}
            </h3>
          </div>

          <!-- Challenge Description -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Challenge Description
            </h3>
            <p class="text-sm text-dojo-text" style="font-family: Inter, sans-serif;">
              {{ challenge.description }}
            </p>
          </div>

          <!-- Repository Origin -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Repository Origin
            </h3>
            <p class="text-xs text-dojo-text opacity-70" style="font-family: monospace;">
              {{ challenge.repoUrl }}
            </p>
          </div>

          <!-- Code Smell Info -->
          <div>
            <h3 class="text-sm font-semibold text-dojo-text mb-2" style="font-family: Inter, sans-serif;">
              Code Smell
            </h3>
            <div class="space-y-2">
              <span class="inline-block px-2 py-0.5 bg-[#F85149]/10 rounded-sm text-xs text-[#F85149] border border-[#F85149]/30">
                {{ challenge.codeSmell }}
              </span>
              <div class="flex items-center gap-2">
                <span class="inline-block px-2 py-0.5 rounded-sm text-xs font-medium {{ difficultyColor(challenge.difficulty) }}">
                  {{ challenge.difficulty }}
                </span>
                <span class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]">
                  {{ challenge.category }}
                </span>
              </div>
            </div>
          </div>
        </div>
      }
    </div>
  `,
})
export class ContextPanelComponent {
  @Input({ required: false }) challenge: Challenge | null = null;

  difficultyColor(difficulty: string): string {
    switch (difficulty) {
      case 'junior':
        return 'bg-green-900 text-green-400 border border-green-700';
      case 'mid':
        return 'bg-yellow-900 text-yellow-400 border border-yellow-700';
      case 'senior':
        return 'bg-orange-900 text-orange-400 border border-orange-700';
      case 'architect':
        return 'bg-red-900 text-red-400 border border-red-700';
      default:
        return 'bg-[#21262D] text-[#8B949E] border border-[#30363D]';
    }
  }
}