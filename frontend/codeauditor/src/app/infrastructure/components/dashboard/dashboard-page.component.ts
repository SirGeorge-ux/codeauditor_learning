import { Component, inject, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterModule } from '@angular/router';
import { LucideActivity } from '@lucide/angular';

import { AuthService } from '../../services/auth.service';
import { ChallengeService } from '../../services/challenge.service';
import { CodeHealthService } from '../../services/code-health.service';

@Component({
  selector: 'app-dashboard-page',
  standalone: true,
  imports: [CommonModule, RouterModule, LucideActivity],
  template: `
    <div class="p-6">
      <!-- Progress Stats Bar -->
      @if (userSignal()) {
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-3xl font-bold text-[#39D353]">{{ userSignal()?.racha_dias ?? 0 }}</div>
            <div class="text-xs text-[#8B949E] mt-1">Día racha</div>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-3xl font-bold text-[#58A6FF]">
              {{ userSignal()?.puntos_maestria ?? 0 }}
            </div>
            <div class="text-xs text-[#8B949E] mt-1">Puntos Maestría</div>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-xl font-bold text-[#C9D1D9]">
              {{ userSignal()?.rango_actual ?? 'Junior' }}
            </div>
            <div class="text-xs text-[#8B949E] mt-1">Rango</div>
          </div>
        </div>
      }

      <h1 class="text-2xl font-bold text-[#C9D1D9] mb-2">Dashboard</h1>
      <p class="text-[#8B949E] mb-8">Welcome to CodeAuditor</p>

      <!-- Practice mode tiles -->
      <div class="mb-6">
        <h2 class="text-sm uppercase tracking-wider text-[#8B949E] mb-3">
          Practice modes
        </h2>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <!-- Code Health tile (S4) -->
          <a
            [routerLink]="['/repo', 'example', 'academy-mic', 'code-health']"
            class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 hover:border-[#39D353] transition-colors flex items-start gap-3"
            data-testid="code-health-tile"
          >
            <svg lucideActivity class="w-7 h-7 text-[#39D353]"></svg>
            <div class="flex-1">
              <h3 class="text-sm font-semibold text-[#C9D1D9] mb-1">Code Health</h3>
              <p class="text-xs text-[#8B949E] mb-2">
                Analyze a Gogs / GitHub repo and rank issues by severity.
              </p>
              <div class="flex items-center gap-2">
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-[10px] text-[#8B949E]"
                >
                  {{ codeHealthScoreLabel() }}
                </span>
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-[10px] text-[#8B949E]"
                >
                  {{ codeHealthRemainingToday() }} left today
                </span>
              </div>
            </div>
          </a>
          <!-- Future tiles (curated, free-practice, ...) live here. -->
        </div>
      </div>

      @if (challengeService.loadingSignal()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Loading challenges...</div>
        </div>
      } @else if (challengeService.challengesSignal().length === 0) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">No challenges available yet.</div>
        </div>
      } @else {
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          @for (challenge of challengeService.challengesSignal(); track challenge.id) {
            <div
              class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 hover:border-blue-400 transition-colors cursor-pointer"
              (click)="navigateToDojo(challenge.id)"
            >
              <h3 class="text-sm font-semibold text-[#C9D1D9] mb-2">{{ challenge.title }}</h3>

              <div class="flex items-center gap-2 mb-3">
                <span
                  class="inline-block px-2 py-0.5 rounded-sm text-xs font-medium {{
                    difficultyColor(challenge.difficulty)
                  }}"
                >
                  {{ challenge.difficulty }}
                </span>
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]"
                >
                  {{ challenge.category }}
                </span>
              </div>

              <div class="flex items-center gap-2 mb-3">
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]"
                >
                  {{ challenge.language }}
                </span>
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#39D353]"
                >
                  {{ challenge.basePoints }} pts
                </span>
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]"
                >
                  {{ challenge.estimatedTimeMinutes }} min
                </span>
              </div>

              <p class="text-xs text-[#8B949E]">
                {{ challenge.description.substring(0, 100)
                }}{{ challenge.description.length > 100 ? '...' : '' }}
              </p>
            </div>
          }
        </div>
      }
    </div>
  `,
})
export class DashboardPageComponent implements OnInit {
  private authService = inject(AuthService);
  private router = inject(Router);
  challengeService = inject(ChallengeService);
  codeHealthService = inject(CodeHealthService);

  userSignal = this.authService.userSignal;

  ngOnInit(): void {
    this.challengeService.loadChallenges();
  }

  navigateToDojo(id: string): void {
    this.router.navigate(['/dojo', id]);
  }

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

  // -- Code Health tile surface ------------------------------------------------
  // The tile is a placeholder link; the dashboard doesn't actually
  // fetch an analysis here (the analysis is owned by the page). We
  // expose a score label so the tile reads as informative, not
  // dangling.

  codeHealthScoreLabel(): string {
    return 'pick a repo';
  }

  codeHealthRemainingToday(): number {
    return this.codeHealthService.remainingToday();
  }
}
