import { Component, signal, inject } from '@angular/core';
import { Router } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { AuthService } from '../services/auth.service';

@Component({
  selector: 'app-register',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="min-h-screen bg-gray-900 flex items-center justify-center p-4">
      <div class="w-full max-w-md">
        <div class="bg-gray-800 rounded-xl shadow-2xl p-8 border border-gray-700">
          <h2 class="text-3xl font-bold text-white mb-8 text-center">Create Account</h2>
          
          @if (successMessage()) {
            <div class="bg-green-500/20 border border-green-500 rounded-lg p-4 mb-6">
              <p class="text-green-400 text-sm">{{ successMessage() }}</p>
            </div>
          }

          @if (errorMessage()) {
            <div class="bg-red-500/20 border border-red-500 rounded-lg p-4 mb-6">
              <p class="text-red-400 text-sm">{{ errorMessage() }}</p>
            </div>
          }

          <form (ngSubmit)="onSubmit()" class="space-y-6">
            <div>
              <label for="email" class="block text-sm font-medium text-gray-300 mb-2">Email</label>
              <input
                type="email"
                id="email"
                [(ngModel)]="email"
                name="email"
                required
                class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="you@example.com"
              />
            </div>

            <div>
              <label for="password" class="block text-sm font-medium text-gray-300 mb-2">Password</label>
              <input
                type="password"
                id="password"
                [(ngModel)]="password"
                name="password"
                required
                minlength="6"
                class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="••••••••"
              />
            </div>

            <div>
              <label for="confirmPassword" class="block text-sm font-medium text-gray-300 mb-2">Confirm Password</label>
              <input
                type="password"
                id="confirmPassword"
                [(ngModel)]="confirmPassword"
                name="confirmPassword"
                required
                class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                placeholder="••••••••"
              />
            </div>

            <button
              type="submit"
              [disabled]="isLoading()"
              class="w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 disabled:cursor-not-allowed text-white font-semibold rounded-lg transition-colors"
            >
              @if (isLoading()) {
                <span>Creating account...</span>
              } @else {
                <span>Create Account</span>
              }
            </button>
          </form>

          <p class="mt-6 text-center text-gray-400 text-sm">
            Already have an account?
            <a routerLink="/login" class="text-blue-400 hover:text-blue-300">Sign in</a>
          </p>
        </div>
      </div>
    </div>
  `
})
export class RegisterComponent {
  private authService = inject(AuthService);
  private router = inject(Router);

  email = '';
  password = '';
  confirmPassword = '';
  isLoading = signal(false);
  errorMessage = signal('');
  successMessage = signal('');

  async onSubmit(): Promise<void> {
    this.errorMessage.set('');
    this.successMessage.set('');

    if (this.password !== this.confirmPassword) {
      this.errorMessage.set('Passwords do not match');
      return;
    }

    if (this.password.length < 6) {
      this.errorMessage.set('Password must be at least 6 characters');
      return;
    }

    this.isLoading.set(true);

    try {
      const { error } = await this.authService.register(this.email, this.password);
      if (error) {
        this.errorMessage.set(error.message || 'Registration failed');
      } else {
        this.successMessage.set('Account created! Check your email for a confirmation link.');
        this.email = '';
        this.password = '';
        this.confirmPassword = '';
      }
    } catch {
      this.errorMessage.set('An unexpected error occurred');
    } finally {
      this.isLoading.set(false);
    }
  }
}