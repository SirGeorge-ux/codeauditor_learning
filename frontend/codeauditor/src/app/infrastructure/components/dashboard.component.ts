import { Component, inject, OnInit, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { AuthService, UserProfile } from '../services/auth.service';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="min-h-screen bg-gray-900">
      <!-- Header -->
      <header class="bg-gray-800 border-b border-gray-700">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div class="flex items-center justify-between">
            <h1 class="text-2xl font-bold text-white">Dashboard</h1>
            <button
              (click)="logout()"
              class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm font-medium rounded-lg transition-colors"
            >
              Sign Out
            </button>
          </div>
        </div>
      </header>

      <!-- Main Content -->
      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        @if (user) {
          <div class="bg-gray-800 rounded-xl shadow-lg p-6 border border-gray-700">
            <h2 class="text-xl font-semibold text-white mb-6">User Profile</h2>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div class="space-y-4">
                <div>
                  <label class="block text-sm font-medium text-gray-400 mb-1">Email</label>
                  <p class="text-white text-lg">{{ user.email }}</p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-400 mb-1">User ID</label>
                  <p class="text-gray-300 text-sm font-mono">{{ user.id }}</p>
                </div>
              </div>

              <div class="space-y-4">
                <div>
                  <label class="block text-sm font-medium text-gray-400 mb-1">Rank</label>
                  <span
                    class="inline-block px-3 py-1 bg-blue-600 text-white text-sm font-semibold rounded-full"
                  >
                    {{ user.rango_actual }}
                  </span>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-400 mb-1">Streak Days</label>
                  <p class="text-white text-2xl font-bold">{{ user.racha_dias }}</p>
                </div>
                <div>
                  <label class="block text-sm font-medium text-gray-400 mb-1">Mastery Points</label>
                  <p class="text-yellow-400 text-2xl font-bold">{{ user.puntos_maestria }}</p>
                </div>
              </div>
            </div>

            <div class="mt-6 pt-6 border-t border-gray-700">
              <p class="text-sm text-gray-400">
                Member since {{ user.created_at | date: 'medium' }}
              </p>
            </div>
          </div>
        } @else {
          <div class="bg-gray-800 rounded-xl shadow-lg p-6 border border-gray-700">
            <p class="text-gray-400">Loading profile...</p>
          </div>
        }
      </main>
    </div>
  `,
})
export class DashboardComponent implements OnInit {
  private authService = inject(AuthService);
  private router = inject(Router);

  user: UserProfile | null = null;

  constructor() {
    effect(() => {
      this.user = this.authService.userSignal();
    });
  }

  ngOnInit(): void {
    this.user = this.authService.userSignal();
  }

  async logout(): Promise<void> {
    await this.authService.logout();
    this.router.navigate(['/login']);
  }
}
