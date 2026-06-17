import { Component, inject, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { AuthService } from '../../services/auth.service';
import { ChallengeService } from '../../services/challenge.service';

@Component({
  selector: 'app-dashboard-page',
  standalone: true,
  imports: [CommonModule],
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
                  class="inline-block px-2 py-0.5 bg-dojo-surface rounded-sm text-xs text-[#F85149] border border-[#F85149]"
                >
                  {{ challenge.codeSmell }}
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
}
