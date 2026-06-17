import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { VaultService, AuditSession, AuditStats } from '../../services/vault.service';
import { ChallengeService } from '../../services/challenge.service';
import { Challenge } from '../../../domain/models/challenge';

@Component({
  selector: 'app-vault-page',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="p-6">
      <h1 class="text-2xl font-bold text-[#C9D1D9] mb-2">Vault</h1>
      <p class="text-[#8B949E] mb-6">Historial de auditorías</p>

      <!-- Stats -->
      @if (stats()) {
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-3xl font-bold text-[#39D353]">{{ stats()?.total_audits ?? 0 }}</div>
            <div class="text-xs text-[#8B949E] mt-1">Auditorías completadas</div>
          </div>
          <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 text-center">
            <div class="text-3xl font-bold text-[#F85149]">{{ stats()?.total_findings ?? 0 }}</div>
            <div class="text-xs text-[#8B949E] mt-1">Hallazgos totales</div>
          </div>
        </div>
      }

      <!-- Loading -->
      @if (loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Cargando historial...</div>
        </div>
      }

      <!-- Error -->
      @if (error()) {
        <div class="bg-red-500/20 border border-red-500 rounded-sm p-4 mb-4">
          <p class="text-red-400 text-sm">{{ error() }}</p>
          <button (click)="load()" class="mt-2 text-xs text-red-300 hover:text-red-200 underline">
            Reintentar
          </button>
        </div>
      }

      <!-- Empty state -->
      @if (!loading() && !error() && sessions().length === 0) {
        <div class="flex items-center justify-center py-12">
          <div class="text-center">
            <div class="text-4xl mb-3">🔒</div>
            <p class="text-[#8B949E]">No hay auditorías todavía.</p>
            <p class="text-[#8B949E] text-sm mt-1">Completá un desafío desde el Dashboard o MCP.</p>
          </div>
        </div>
      }

      <!-- Session list -->
      @if (!loading() && sessions().length > 0) {
        <div class="space-y-3">
          @for (session of sessions(); track session.id) {
            <div
              class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 hover:border-blue-400 transition-colors cursor-pointer"
              (click)="reAudit(session)"
            >
              <div class="flex items-center justify-between mb-2">
                <h3 class="text-sm font-semibold text-[#C9D1D9]">
                  {{ session.challenge_title || 'Custom Audit' }}
                </h3>
                <span class="text-xs text-[#8B949E]">{{
                  session.created_at | date: 'dd/MM/yy HH:mm'
                }}</span>
              </div>
              <div class="flex items-center gap-2 mb-2">
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]"
                >
                  {{ session.language }}
                </span>
                <span
                  class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#F85149]"
                >
                  {{ session.findings_count }} hallazgos
                </span>
              </div>
              <pre
                class="text-xs text-[#8B949E] bg-[#0D1117] rounded-sm p-2 overflow-hidden max-h-16"
                >{{ session.code_snippet }}</pre
              >
            </div>
          }
        </div>
      }
    </div>
  `,
})
export class VaultPageComponent implements OnInit {
  private vaultService = inject(VaultService);
  private challengeService = inject(ChallengeService);
  private router = inject(Router);

  sessions = signal<AuditSession[]>([]);
  stats = signal<AuditStats | null>(null);
  loading = signal(true);
  error = signal('');

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.error.set('');

    this.vaultService.getStats().subscribe({
      next: (s) => this.stats.set(s),
      error: () => {},
    });

    this.vaultService.getHistory().subscribe({
      next: (sessions) => {
        this.sessions.set(sessions);
        this.loading.set(false);
      },
      error: (_err) => {
        this.error.set('No se pudo cargar el historial');
        this.loading.set(false);
      },
    });
  }

  reAudit(session: AuditSession): void {
    const challenge: Challenge = {
      id: 'vault-' + session.id,
      title: session.challenge_title,
      description: 'Re-auditoría del ' + new Date(session.created_at).toLocaleDateString(),
      difficulty: 'mid',
      category: 'vault',
      language: session.language,
      repoUrl: '',
      code: session.code_snippet,
      codeSmell: 'pending-analysis',
      status: 'available',
      createdAt: new Date(session.created_at),
    };

    const tempId = this.challengeService.addTempChallenge(challenge);
    this.router.navigate(['/dojo', tempId]);
  }
}
