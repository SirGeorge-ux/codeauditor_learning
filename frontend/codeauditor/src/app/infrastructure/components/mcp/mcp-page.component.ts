// McpPageComponent — smart component for browsing Gogs repos and importing files as challenges.
//
// Loads repo list on init, lets user select a repo, enter a file path,
// fetch the file content, and create a temporary Challenge for the Dojo.
import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { GogsService, GogsRepo } from '../../services/gogs.service';
import { ChallengeService } from '../../services/challenge.service';

@Component({
  selector: 'app-mcp-page',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="p-6">
      <h1 class="text-2xl font-bold text-[#C9D1D9] mb-2">Repositorios</h1>
      <p class="text-[#8B949E] mb-6">Seleccion&aacute; un repositorio para auditar su c&oacute;digo</p>

      @if (loading()) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">Cargando repositorios...</div>
        </div>
      } @else if (error()) {
        <div class="flex flex-col items-center justify-center py-12">
          <div class="text-[#F85149] mb-4">{{ error() }}</div>
          <button
            (click)="loadRepos()"
            class="px-4 py-2 bg-[#21262D] text-[#C9D1D9] rounded-sm hover:bg-[#30363D] transition-colors"
          >
            Reintentar
          </button>
        </div>
      } @else if (repos().length === 0) {
        <div class="flex items-center justify-center py-12">
          <div class="text-[#8B949E]">No repositories found. Connect a Gogs account to get started.</div>
        </div>
      } @else if (selectedRepo()) {
        <!-- File browser view -->
        <div class="mb-4">
          <button
            (click)="deselectRepo()"
            class="text-[#8B949E] hover:text-[#C9D1D9] transition-colors text-sm"
          >
            &larr; Volver a repositorios
          </button>
        </div>

        <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 mb-4">
          <h2 class="text-lg font-semibold text-[#C9D1D9] mb-1">{{ selectedRepo()!.full_name }}</h2>
          <p class="text-sm text-[#8B949E] mb-3">{{ selectedRepo()!.description || 'Sin descripci&oacute;n' }}</p>
          <span class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]">
            {{ selectedRepo()!.default_branch }}
          </span>
        </div>

        <div class="bg-[#161B22] border border-[#21262D] rounded-sm p-4">
          <label class="block text-sm text-[#8B949E] mb-2">Ruta del archivo</label>
          <div class="flex gap-2">
            <input
              type="text"
              [(ngModel)]="filePath"
              placeholder="e.g., src/main.go"
              class="flex-1 bg-[#0D1117] border border-[#21262D] rounded-sm px-3 py-2 text-[#C9D1D9] placeholder-[#8B949E] focus:outline-none focus:border-[#39D353]"
            />
            <button
              (click)="fetchAndAudit()"
              [disabled]="!filePath.trim() || fetchingFile()"
              class="px-4 py-2 rounded-sm transition-colors"
              [class.bg-[#39D353]]="filePath.trim() && !fetchingFile()"
              [class.text-[#0D1117]]="filePath.trim() && !fetchingFile()"
              [class.bg-[#21262D]]="!filePath.trim() || fetchingFile()"
              [class.text-[#8B949E]]="!filePath.trim() || fetchingFile()"
              [class.cursor-not-allowed]="!filePath.trim() || fetchingFile()"
            >
              @if (fetchingFile()) {
                Cargando...
              } @else {
                Auditar
              }
            </button>
          </div>

          @if (fileError()) {
            <div class="mt-3">
              <p class="text-[#F85149] text-sm">{{ fileError() }}</p>
              @if (fileError() !== 'This file exceeds the maximum size of 1 MB. Please select a smaller file.') {
                <button
                  (click)="fetchAndAudit()"
                  class="mt-2 text-sm text-[#8B949E] hover:text-[#C9D1D9] underline"
                >
                  Reintentar
                </button>
              }
            </div>
          }
        </div>
      } @else {
        <!-- Repo list -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          @for (repo of repos(); track repo.id) {
            <div
              class="bg-[#161B22] border border-[#21262D] rounded-sm p-4 hover:border-[#39D353] transition-colors cursor-pointer"
              (click)="selectRepo(repo)"
            >
              <h3 class="text-sm font-semibold text-[#C9D1D9] mb-2">{{ repo.full_name }}</h3>
              @if (repo.description) {
                <p class="text-xs text-[#8B949E] mb-3">{{ repo.description }}</p>
              }
              <div class="flex items-center gap-2">
                <span class="inline-block px-2 py-0.5 bg-[#21262D] rounded-sm text-xs text-[#8B949E]">
                  {{ repo.default_branch }}
                </span>
              </div>
            </div>
          }
        </div>
      }
    </div>
  `,
})
export class McpPageComponent implements OnInit {
  private gogsService = inject(GogsService);
  private challengeService = inject(ChallengeService);
  private router = inject(Router);

  repos = signal<GogsRepo[]>([]);
  loading = signal(true);
  error = signal<string | null>(null);
  selectedRepo = signal<GogsRepo | null>(null);
  filePath = '';
  fetchingFile = signal(false);
  fileError = signal<string | null>(null);

  ngOnInit(): void {
    this.loadRepos();
  }

  loadRepos(): void {
    this.loading.set(true);
    this.error.set(null);
    this.gogsService.listRepos().subscribe({
      next: (repos) => {
        this.repos.set(repos);
        this.loading.set(false);
      },
      error: (err) => {
        this.error.set(err.error?.error || err.message || 'Gogs service is unavailable');
        this.loading.set(false);
      },
    });
  }

  selectRepo(repo: GogsRepo): void {
    this.selectedRepo.set(repo);
    this.filePath = '';
    this.fileError.set(null);
  }

  deselectRepo(): void {
    this.selectedRepo.set(null);
    this.filePath = '';
    this.fileError.set(null);
  }

  fetchAndAudit(): void {
    const repo = this.selectedRepo();
    if (!repo || !this.filePath.trim()) return;

    this.fetchingFile.set(true);
    this.fileError.set(null);

    const owner = repo.full_name.split('/')[0];

    this.gogsService.fetchFile(owner, repo.name, repo.default_branch, this.filePath.trim()).subscribe({
      next: (response) => {
        this.fetchingFile.set(false);
        const decodedCode = atob(response.content);
        const tempId = this.challengeService.addTempChallenge({
          id: '',
          title: response.path.split('/').pop() || response.path,
          description: `Imported from ${response.owner}/${response.repo}:${response.branch}`,
          difficulty: 'mid',
          category: 'imported',
          language: response.language,
          repoUrl: response.path,
          code: decodedCode,
          codeSmell: 'pending-analysis',
          status: 'available',
          createdAt: new Date(),
        });
        this.router.navigate(['/dojo', tempId]);
      },
      error: (err) => {
        this.fetchingFile.set(false);
        this.fileError.set(err.error?.error || err.message || 'Failed to fetch file');
      },
    });
  }
}