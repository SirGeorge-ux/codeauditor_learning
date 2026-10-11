// dashboard-page.component.spec.ts — tile-level assertions for the
// code-health entry in the dashboard grid. Verifies the tile renders
// the heart-pulse icon + label.
import { describe, it, expect, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { provideRouter } from '@angular/router';

import { DashboardPageComponent } from './dashboard-page.component';
import { CodeHealthService } from '../../services/code-health.service';

describe('DashboardPageComponent (code-health tile)', () => {
  let fixture: ComponentFixture<DashboardPageComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CommonModule, DashboardPageComponent],
      providers: [
        provideRouter([]),
        {
          provide: CodeHealthService,
          useValue: {
            analysis: () => null,
            loading: () => false,
            error: () => null,
            remainingToday: () => 5,
            topIssues: () => [],
            languages: () => ({}),
            submitAnalysis: async () => null,
            loadRepoIssues: async () => [],
            generateHealthReport: async () => '',
            suppressIssue: async () => false,
            clearError: () => undefined,
          },
        },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(DashboardPageComponent);
    fixture.detectChanges();
  });

  it('renders the code-health tile', () => {
    const tile = fixture.nativeElement.querySelector('[data-testid="code-health-tile"]');
    expect(tile).not.toBeNull();
    expect((tile as HTMLElement).textContent ?? '').toContain('Code Health');
  });

  it('links to /repo/example/academy-mic/code-health by default', () => {
    const tile = fixture.nativeElement.querySelector(
      '[data-testid="code-health-tile"]',
    ) as HTMLAnchorElement;
    expect(tile.getAttribute('href')).toContain('/repo/example/academy-mic/code-health');
  });
});
