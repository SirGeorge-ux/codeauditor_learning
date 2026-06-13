import { Component, inject, OnInit, signal, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { ContextPanelComponent } from '../shared/context-panel.component';
import { CodePanelComponent } from '../shared/code-panel.component';
import { TerminalPanelComponent } from '../shared/terminal-panel.component';
import { ResizeDirective } from '../shared/resize.directive';
import { AuditService } from '../../services/audit.service';
import { ChallengeService } from '../../services/challenge.service';

@Component({
  selector: 'app-dojo-page',
  standalone: true,
  imports: [
    CommonModule,
    ContextPanelComponent,
    CodePanelComponent,
    TerminalPanelComponent,
    ResizeDirective,
  ],
  template: `
    <!-- Top bar -->
    <div class="h-10 flex items-center justify-between px-4 bg-[#161B22] border-b border-[#21262D]">
      <span class="text-xs text-[#8B949E]">
        @if (challengeService.selectedChallengeSignal()) {
          Auditando: {{ challengeService.selectedChallengeSignal()?.title }}
        } @else {
          No challenge selected
        }
      </span>
      <button
        (click)="auditChallenge()"
        [disabled]="!challengeService.selectedChallengeSignal() || isAuditing()"
        class="px-3 py-1 text-xs rounded-sm transition-colors"
        [class.bg-blue-600]="!isAuditing()"
        [class.bg-blue-800]="isAuditing()"
        [class.text-white]="true"
        [class.opacity-50]="!challengeService.selectedChallengeSignal()"
        [class.cursor-not-allowed]="!challengeService.selectedChallengeSignal()">
        @if (isAuditing()) {
          Auditando...
        } @else {
          Auditar
        }
      </button>
    </div>

    <div class="h-full flex bg-[#0D1117]">
      @if (challengeService.loadingSignal()) {
        <div class="flex-1 flex items-center justify-center">
          <div class="text-[#8B949E]">Loading challenge...</div>
        </div>
      } @else if (challengeService.selectedChallengeSignal() === null && hasId) {
        <div class="flex-1 flex items-center justify-center">
          <div class="text-[#F85149]">Challenge not found</div>
        </div>
      } @else {
        <!-- Left: Context Panel (resizable) -->
        <div
          class="overflow-hidden border-r border-[#21262D]"
          appResize
          [minWidth]="250"
          [maxWidth]="500"
          [initialWidth]="300"
        >
          <app-context-panel [challenge]="challengeService.selectedChallengeSignal()"></app-context-panel>
        </div>

        <!-- Right: Code + Terminal split -->
        <div class="flex-1 flex flex-col overflow-hidden">
          <div class="flex-1 overflow-hidden border-b border-[#21262D]">
            <app-code-panel
              [code]="challengeService.selectedChallengeSignal()?.code ?? ''"
              [language]="challengeService.selectedChallengeSignal()?.language ?? 'typescript'"
              [readOnly]="true"
            ></app-code-panel>
          </div>
          <div class="h-48 overflow-hidden">
            <app-terminal-panel #termPanel></app-terminal-panel>
          </div>
        </div>
      }
    </div>
  `,
})
export class DojoPageComponent implements OnInit {
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  challengeService = inject(ChallengeService);
  private auditService = inject(AuditService);

  @ViewChild('termPanel', { static: false }) terminalPanel!: TerminalPanelComponent;

  hasId = false;
  isAuditing = signal(false);

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
    if (id) {
      this.hasId = true;
      this.challengeService.selectChallenge(id);
    }
  }

  auditChallenge(): void {
    const challenge = this.challengeService.selectedChallengeSignal();
    if (!challenge || this.isAuditing()) return;

    this.isAuditing.set(true);
    this.terminalPanel?.clear();
    this.terminalPanel?.write('\x1b[36mStarting audit...\x1b[0m\n\r');

    this.auditService.runAudit(
      challenge.code,
      challenge.language,
      challenge.id
    ).subscribe({
      next: (event) => {
        if (event.type === 'stdout') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write((payload.data ?? event.data) + '\n\r');
          } catch {
            this.terminalPanel?.write(event.data + '\n\r');
          }
        } else if (event.type === 'stderr') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write('\x1b[33m' + (payload.data ?? event.data) + '\x1b[0m\n\r');
          } catch {
            this.terminalPanel?.write('\x1b[33m' + event.data + '\x1b[0m\n\r');
          }
        } else if (event.type === 'error') {
          try {
            const payload = JSON.parse(event.data);
            this.terminalPanel?.write('\x1b[31mError: ' + (payload.message ?? event.data) + '\x1b[0m\n\r');
          } catch {
            this.terminalPanel?.write('\x1b[31mError: ' + event.data + '\x1b[0m\n\r');
          }
        }
      },
      error: (err) => {
        this.terminalPanel?.write('\x1b[31mError: ' + err.message + '\x1b[0m\n\r');
        this.isAuditing.set(false);
      },
      complete: () => {
        this.terminalPanel?.write('\n\r\x1b[32mAudit complete.\x1b[0m\n\r');
        this.isAuditing.set(false);
      },
    });
  }
}