import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { LucideUser, LucideTrendingUp, LucideSettings, LucideBookOpen } from '@lucide/angular';

import { AuthService } from '../../services/auth.service';
import { ProgressService } from '../../services/progress.service';

@Component({
  selector: 'app-profile-page',
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    LucideUser,
    LucideTrendingUp,
    LucideSettings,
    LucideBookOpen,
  ],
  template: `
    <div class="p-6 max-w-5xl mx-auto">
      <!-- Header -->
      <div class="flex items-center gap-3 mb-8">
        <svg lucideUser class="w-7 h-7 text-[#8B949E]"></svg>
        <div>
          <h1 class="text-2xl font-bold text-[#C9D1D9]">Profile</h1>
          <p class="text-sm text-[#8B949E]">Your language progress and learning preferences</p>
        </div>
      </div>

      <!-- User Info Bar -->
      @if (userSignal()) {
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-lg font-bold text-[#C9D1D9]">
              {{ userSignal()?.display_name || userSignal()?.email || 'User' }}
            </div>
            <div class="text-xs text-[#8B949E] mt-1">{{ userSignal()?.email }}</div>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-2xl font-bold text-[#39D353]">{{ useCase.rangoGlobal() }}</div>
            <div class="text-xs text-[#8B949E] mt-1">Global Rank (F-S)</div>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-xl font-bold text-[#C9D1D9]">
              {{ userSignal()?.rango_actual ?? 'Junior' }}
            </div>
            <div class="text-xs text-[#8B949E] mt-1">Rango Actual</div>
          </div>
        </div>
      }

      <!-- Loading State -->
      @if (useCase.loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Loading progress...</div>
        </div>
      }

      <!-- Error State -->
      @if (useCase.error()) {
        <div class="bg-[#161B22] border border-[#F85149] rounded-sm p-4 mb-8">
          <p class="text-sm text-[#F85149]">{{ useCase.error() }}</p>
        </div>
      }

      <!-- Language Cards -->
      @if (!useCase.loading() && useCase.languages().length > 0) {
        <div class="mb-8">
          <div class="flex items-center gap-2 mb-4">
            <svg lucideTrendingUp class="w-5 h-5 text-[#39D353]"></svg>
            <h2 class="text-lg font-semibold text-[#C9D1D9]">Language Progress</h2>
          </div>
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            @for (lang of useCase.languages(); track lang.language) {
              <div
                class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 hover:border-[#30363D] transition-colors"
              >
                <!-- Language name + badges -->
                <div class="flex items-center justify-between mb-3">
                  <h3 class="text-sm font-semibold text-[#C9D1D9] capitalize">
                    {{ lang.language }}
                  </h3>
                  <div class="flex items-center gap-1.5">
                    <span
                      class="inline-block px-2 py-0.5 rounded-sm text-xs font-bold {{
                        fBadgeColor(lang.rango)
                      }}"
                    >
                      {{ lang.rango }}
                    </span>
                    <span
                      class="inline-block px-2 py-0.5 rounded-sm text-xs font-medium {{
                        jrArchBadgeColor(lang.rangoCodeHealth)
                      }}"
                    >
                      {{ lang.rangoCodeHealth }}
                    </span>
                  </div>
                </div>

                <!-- Stats -->
                <div class="grid grid-cols-2 gap-2 mb-3">
                  <div class="text-xs">
                    <span class="text-[#8B949E]">Points</span>
                    <span class="block text-[#39D353] font-semibold">{{ lang.puntos }}</span>
                  </div>
                  <div class="text-xs">
                    <span class="text-[#8B949E]">Success rate</span>
                    <span class="block text-[#C9D1D9] font-semibold">
                      {{ lang.tasaExito }}%
                    </span>
                  </div>
                  <div class="text-xs">
                    <span class="text-[#8B949E]">Completed</span>
                    <span class="block text-[#C9D1D9] font-semibold">
                      {{ lang.challengesCompletados }}
                    </span>
                  </div>
                  <div class="text-xs">
                    <span class="text-[#8B949E]">Attempted</span>
                    <span class="block text-[#C9D1D9] font-semibold">
                      {{ lang.challengesIntentados }}
                    </span>
                  </div>
                </div>

                <!-- Code health stats -->
                @if (lang.puntosCodeHealth > 0 || lang.reposAnalizados > 0) {
                  <div class="border-t border-[#21262D] pt-2 mb-3">
                    <div class="text-xs text-[#8B949E] mb-1">Code Health</div>
                    <div class="grid grid-cols-2 gap-2">
                      <div class="text-xs">
                        <span class="text-[#8B949E]">Points</span>
                        <span class="block text-[#D29922] font-semibold">
                          {{ lang.puntosCodeHealth }}
                        </span>
                      </div>
                      <div class="text-xs">
                        <span class="text-[#8B949E]">Repos</span>
                        <span class="block text-[#C9D1D9] font-semibold">
                          {{ lang.reposAnalizados }}
                        </span>
                      </div>
                    </div>
                  </div>
                }

                <!-- Topics mastered -->
                @if (lang.topicsDominados.length > 0) {
                  <div class="border-t border-[#21262D] pt-2">
                    <div class="text-xs text-[#8B949E] mb-1">Topics dominados</div>
                    <div class="flex flex-wrap gap-1">
                      @for (topic of lang.topicsDominados; track topic) {
                        <span
                          class="inline-block px-1.5 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]"
                        >
                          {{ topic }}
                        </span>
                      }
                    </div>
                  </div>
                }
              </div>
            }
          </div>
        </div>
      }

      <!-- Learning Profile Section -->
      @if (useCase.learningProfile()) {
        <div class="mb-8">
          <div class="flex items-center gap-2 mb-4">
            <svg lucideSettings class="w-5 h-5 text-[#8B949E]"></svg>
            <h2 class="text-lg font-semibold text-[#C9D1D9]">Learning Preferences</h2>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 max-w-md">
            <!-- Idioma selector -->
            <div class="mb-4">
              <label class="text-xs text-[#8B949E] block mb-1">Idioma del tutor</label>
              <select
                class="w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
                [value]="useCase.learningProfile()!.preferencias.idioma"
                (change)="onIdiomaChange($event)"
              >
                <option value="es">Espanol</option>
                <option value="en">English</option>
                <option value="fr">Francais</option>
                <option value="de">Deutsch</option>
              </select>
            </div>

            <!-- Socratismo level -->
            <div class="mb-4">
              <label class="text-xs text-[#8B949E] block mb-1">
                Nivel de socratismo: {{ useCase.learningProfile()!.preferencias.nivelSocratismo }}
                <span class="text-[#8B949E]">({{ socratismoLabel(useCase.learningProfile()!.preferencias.nivelSocratismo) }})</span>
              </label>
              <input
                type="range"
                min="0"
                max="3"
                step="1"
                class="w-full accent-[#39D353]"
                [value]="useCase.learningProfile()!.preferencias.nivelSocratismo"
                (input)="onSocratismoChange($event)"
              />
              <div class="flex justify-between text-xs text-[#8B949E] mt-1">
                <span>Directa</span>
                <span>Descubrimiento</span>
              </div>
            </div>

            <!-- Learning style -->
            <div class="mb-4">
              <label class="text-xs text-[#8B949E] block mb-1">Aprende mejor con</label>
              <select
                class="w-full bg-[#0D1117] border border-[#30363D] rounded-sm px-3 py-2 text-sm text-[#C9D1D9] focus:border-[#39D353] focus:outline-none"
                [value]="useCase.learningProfile()!.estiloAprendizaje.aprendeMejorCon"
                (change)="onEstiloChange($event)"
              >
                <option value="ejemplos">Ejemplos</option>
                <option value="teoria">Teoria</option>
                <option value="practica">Practica</option>
                <option value="analogias">Analogias</option>
              </select>
            </div>

            <!-- Save button -->
            <button
              class="w-full bg-[#238636] hover:bg-[#2EA043] text-white text-sm font-semibold py-2 rounded-sm transition-colors disabled:opacity-50"
              [disabled]="saving()"
              (click)="saveProfile()"
            >
              {{ saving() ? 'Saving...' : 'Save preferences' }}
            </button>
          </div>
        </div>
      }

      <!-- Empty State -->
      @if (!useCase.loading() && useCase.languages().length === 0 && !useCase.error()) {
        <div class="flex flex-col items-center justify-center py-16">
          <svg lucideBookOpen class="w-12 h-12 text-[#30363D] mb-4"></svg>
          <p class="text-lg text-[#8B949E] mb-2">No practice data yet.</p>
          <a
            routerLink="/dojo"
            class="text-sm text-[#39D353] hover:text-[#2EA043] transition-colors"
          >
            Complete your first challenge!
          </a>
        </div>
      }
    </div>
  `,
})
export class ProfilePageComponent implements OnInit {
  private authService = inject(AuthService);
  useCase = inject(ProgressService);
  saving = signal(false);

  userSignal = this.authService.userSignal;

  ngOnInit(): void {
    this.useCase.loadProgress();
    this.useCase.loadLearningProfile();
  }

  onIdiomaChange(event: Event): void {
    const profile = this.useCase.learningProfile();
    if (!profile) return;
    const value = (event.target as HTMLSelectElement).value;
    // Update model in-place via signal
    this.useCase.updatePreferencias({ ...profile.preferencias, idioma: value });
  }

  onSocratismoChange(event: Event): void {
    const profile = this.useCase.learningProfile();
    if (!profile) return;
    const value = Number((event.target as HTMLInputElement).value);
    this.useCase.updatePreferencias({ ...profile.preferencias, nivelSocratismo: value });
  }

  onEstiloChange(event: Event): void {
    const profile = this.useCase.learningProfile();
    if (!profile) return;
    const value = (event.target as HTMLSelectElement).value;
    this.useCase.updateEstilo({ aprendeMejorCon: value });
  }

  async saveProfile(): Promise<void> {
    this.saving.set(true);
    const profile = this.useCase.learningProfile();
    if (profile) {
      await this.useCase.saveLearningProfile(
        profile.preferencias,
        profile.estiloAprendizaje,
      );
    }
    this.saving.set(false);
  }

  socratismoLabel(level: number): string {
    switch (level) {
      case 0:
        return 'Directa';
      case 1:
        return 'Guiada';
      case 2:
        return 'Socratica';
      case 3:
        return 'Descubrimiento';
      default:
        return '';
    }
  }

  fBadgeColor(rango: string): string {
    switch (rango) {
      case 'F':
        return 'bg-gray-800 text-gray-400 border border-gray-600';
      case 'E':
        return 'bg-blue-900 text-blue-400 border border-blue-700';
      case 'D':
        return 'bg-green-900 text-green-400 border border-green-700';
      case 'C':
        return 'bg-yellow-900 text-yellow-400 border border-yellow-700';
      case 'B':
        return 'bg-orange-900 text-orange-400 border border-orange-700';
      case 'A':
        return 'bg-purple-900 text-purple-400 border border-purple-700';
      case 'S':
        return 'bg-yellow-600 text-black border border-yellow-400';
      default:
        return 'bg-[#21262D] text-[#8B949E] border border-[#30363D]';
    }
  }

  jrArchBadgeColor(rango: string): string {
    switch (rango) {
      case 'Junior':
        return 'bg-gray-800 text-gray-400 border border-gray-600';
      case 'Mid':
        return 'bg-blue-900 text-blue-400 border border-blue-700';
      case 'Senior':
        return 'bg-green-900 text-green-400 border border-green-700';
      case 'Architect':
        return 'bg-purple-900 text-purple-400 border border-purple-700';
      default:
        return 'bg-[#21262D] text-[#8B949E] border border-[#30363D]';
    }
  }
}