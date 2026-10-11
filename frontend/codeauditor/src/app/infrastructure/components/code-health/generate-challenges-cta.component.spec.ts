// generate-challenges-cta.component.spec.ts — per-issue CTA that
// turns a code-health issue into a dojo challenge. Calls
// `mcp-challenges.generate_challenge({sourceIssueId, language})` and
// navigates to `/dojo/<id>` on success. Mirrors the PR 2 spec
// (SourceIssueID + origin="code_health" persistence).
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { provideRouter, Router } from '@angular/router';
import { of, throwError } from 'rxjs';

import { GenerateChallengesCtaComponent } from './generate-challenges-cta.component';
import { CodeHealthIssue } from '../../../domain/models/code-health';
import { MCPClientPort, MCPTool } from '../../../domain/ports/mcp-client.port';
import { MCPClientAdapter } from '../../../infrastructure/adapters/mcp-client.adapter';

class FakeMcp implements MCPClientPort {
  captured: { name: string; args: unknown }[] = [];
  challengeResult: { id: string } = { id: 'gen-1' };
  challengeError: Error | null = null;

  listTools() {
    return of([] as MCPTool[]);
  }
  callTool<T = unknown>(name: string, args: unknown) {
    this.captured.push({ name, args });
    if (this.challengeError) {
      return throwError(() => this.challengeError);
    }
    if (name === 'mcp-challenges.generate_challenge') {
      return of(this.challengeResult as unknown as T);
    }
    return of({} as T);
  }
  streamTool<T = unknown>(_name: string, _args: unknown) {
    return of({} as T);
  }
}

describe('GenerateChallengesCtaComponent', () => {
  let fixture: ComponentFixture<GenerateChallengesCtaComponent>;
  let component: GenerateChallengesCtaComponent;
  let mcp: FakeMcp;
  let navSpy: ReturnType<typeof vi.fn>;

  const issue: CodeHealthIssue = {
    id: 'i-1',
    rule_id: 'RULE-SEC-SECRET-001',
    category: 'security',
    severity: 'critical',
    message: 'm',
    file_path: 'src/auth.ts',
    line_number: 14,
    evidence: 'ev',
    suggested_fix: 'fix',
    confidence: 1.0,
  };

  beforeEach(async () => {
    mcp = new FakeMcp();
    navSpy = vi.fn();
    await TestBed.configureTestingModule({
      imports: [CommonModule, GenerateChallengesCtaComponent],
      providers: [
        provideRouter([]),
        {
          provide: Router,
          useValue: { navigate: navSpy },
        },
        {
          provide: MCPClientAdapter,
          useValue: mcp,
        },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(GenerateChallengesCtaComponent);
    component = fixture.componentInstance;
  });

  it('renders a button labelled "Generate challenge"', () => {
    fixture.componentRef.setInput('issue', issue);
    fixture.componentRef.setInput('language', 'typescript');
    fixture.detectChanges();
    const btn = fixture.nativeElement.querySelector(
      '[data-testid="generate-cta"]',
    ) as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.textContent).toContain('Generate challenge');
  });

  it('calls mcp-challenges.generate_challenge with sourceIssueId + language', async () => {
    fixture.componentRef.setInput('issue', issue);
    fixture.componentRef.setInput('language', 'typescript');
    fixture.detectChanges();
    await component.invoke();
    const call = mcp.captured.find((c) => c.name === 'mcp-challenges.generate_challenge');
    expect(call?.args).toEqual({
      sourceIssueId: 'i-1',
      language: 'typescript',
    });
  });

  it('navigates to /dojo/<id> on success', async () => {
    mcp.challengeResult = { id: 'gen-99' };
    fixture.componentRef.setInput('issue', issue);
    fixture.componentRef.setInput('language', 'typescript');
    fixture.detectChanges();
    await component.invoke();
    expect(navSpy).toHaveBeenCalledWith(['/dojo', 'gen-99']);
  });

  it('surfaces backend errors via the error signal', async () => {
    mcp.challengeError = new Error('sandbox failed');
    fixture.componentRef.setInput('issue', issue);
    fixture.componentRef.setInput('language', 'typescript');
    fixture.detectChanges();
    await component.invoke();
    expect(component.error()).toMatch(/sandbox failed/i);
  });
});
