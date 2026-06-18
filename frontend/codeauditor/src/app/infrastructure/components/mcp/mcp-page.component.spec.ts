import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { of, throwError } from 'rxjs';
import { HttpErrorResponse } from '@angular/common/http';

import { McpPageComponent } from './mcp-page.component';
import { GogsService, GogsRepo, GogsFileResponse } from '../../services/gogs.service';
import { ChallengeService } from '../../services/challenge.service';

describe('McpPageComponent', () => {
  let component: McpPageComponent;
  let fixture: ComponentFixture<McpPageComponent>;
  let gogsServiceMock: {
    listRepos: ReturnType<typeof vi.fn>;
    fetchFile: ReturnType<typeof vi.fn>;
  };
  let challengeServiceMock: {
    importChallenge: ReturnType<typeof vi.fn>;
  };
  let routerMock: {
    navigate: ReturnType<typeof vi.fn>;
  };

  const mockRepos: GogsRepo[] = [
    {
      id: 1,
      name: 'test-repo',
      full_name: 'org/test-repo',
      description: 'A test repository',
      private: false,
      clone_url: 'https://gogs.example.com/org/test-repo.git',
      default_branch: 'main',
    },
    {
      id: 2,
      name: 'another-repo',
      full_name: 'org/another-repo',
      description: '',
      private: true,
      clone_url: 'https://gogs.example.com/org/another-repo.git',
      default_branch: 'develop',
    },
  ];

  const mockFileResponse: GogsFileResponse = {
    owner: 'org',
    repo: 'test-repo',
    branch: 'main',
    path: 'src/main.go',
    content: btoa('package main\n\nfunc main() {}'),
    encoding: 'base64',
    language: 'go',
    size: 30,
  };

  beforeEach(async () => {
    gogsServiceMock = {
      listRepos: vi.fn(),
      fetchFile: vi.fn(),
    };
    challengeServiceMock = {
      importChallenge: vi.fn().mockResolvedValue('ch-new-id'),
    };
    routerMock = {
      navigate: vi.fn(),
    };

    // Default: return repos successfully
    gogsServiceMock.listRepos.mockReturnValue(of(mockRepos));

    await TestBed.configureTestingModule({
      imports: [McpPageComponent],
      providers: [
        { provide: GogsService, useValue: gogsServiceMock },
        { provide: ChallengeService, useValue: challengeServiceMock },
        { provide: Router, useValue: routerMock },
        provideRouter([]),
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(McpPageComponent);
    component = fixture.componentInstance;
  });

  it('should create the component', () => {
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('should load repos on init', () => {
    fixture.detectChanges();
    expect(gogsServiceMock.listRepos).toHaveBeenCalledOnce();
    expect(component.repos()).toEqual(mockRepos);
    expect(component.loading()).toBe(false);
  });

  it('should display repo cards after loading', () => {
    fixture.detectChanges();
    // At least repo cards rendered
    expect(component.repos().length).toBe(2);
  });

  it('should show empty state when no repos are returned', () => {
    gogsServiceMock.listRepos.mockReturnValue(of([]));
    fixture.detectChanges();
    expect(component.repos()).toEqual([]);
    expect(component.loading()).toBe(false);
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('No repositories found');
  });

  it('should show error state with retry when listRepos fails', () => {
    gogsServiceMock.listRepos.mockReturnValue(
      throwError(
        () =>
          new HttpErrorResponse({
            error: { error: 'Gogs service is unavailable' },
            status: 503,
            statusText: 'Service Unavailable',
          }),
      ),
    );
    fixture.detectChanges();
    expect(component.error()).toBeTruthy();
    expect(component.loading()).toBe(false);
  });

  it('should select a repo and show file path input', () => {
    fixture.detectChanges();
    component.selectRepo(mockRepos[0]);
    fixture.detectChanges();
    expect(component.selectedRepo()).toEqual(mockRepos[0]);
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Ruta del archivo');
  });

  it('should deselect repo and return to repo list', () => {
    fixture.detectChanges();
    component.selectRepo(mockRepos[0]);
    fixture.detectChanges();
    component.deselectRepo();
    fixture.detectChanges();
    expect(component.selectedRepo()).toBeNull();
  });

  it('should import challenge and navigate to dojo on successful file fetch', async () => {
    gogsServiceMock.fetchFile.mockReturnValue(of(mockFileResponse));
    fixture.detectChanges();
    component.selectRepo(mockRepos[0]);
    component.filePath = 'src/main.go';
    component.fetchAndAudit();

    // Wait for async operations
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(gogsServiceMock.fetchFile).toHaveBeenCalledWith(
      'org',
      'test-repo',
      'main',
      'src/main.go',
    );
    expect(challengeServiceMock.importChallenge).toHaveBeenCalledOnce();

    const importArg = challengeServiceMock.importChallenge.mock.calls[0][0];
    expect(importArg.difficulty).toBe('mid');
    expect(importArg.category).toBe('imported');
    expect(importArg.language).toBe('go');
    expect(importArg.codeSmell).toBe('pending-analysis');
    expect(importArg.code).toBe('package main\n\nfunc main() {}');
    expect(importArg.repoUrl).toBe('src/main.go');
    expect(importArg.sourceRepo).toBe('org/test-repo');

    expect(routerMock.navigate).toHaveBeenCalledWith(['/dojo', 'ch-new-id']);
  });

  it('should show file error when fetchFile fails', () => {
    gogsServiceMock.fetchFile.mockReturnValue(
      throwError(
        () =>
          new HttpErrorResponse({
            error: { error: 'File not found' },
            status: 404,
            statusText: 'Not Found',
          }),
      ),
    );
    fixture.detectChanges();
    component.selectRepo(mockRepos[0]);
    component.filePath = 'nonexistent.go';
    component.fetchAndAudit();

    expect(component.fileError()).toBeTruthy();
    expect(component.fetchingFile()).toBe(false);
  });

  it('should not call fetchFile when filePath is empty', () => {
    fixture.detectChanges();
    component.selectRepo(mockRepos[0]);
    component.filePath = '';
    component.fetchAndAudit();
    expect(gogsServiceMock.fetchFile).not.toHaveBeenCalled();
  });

  it('should not call fetchFile when no repo is selected', () => {
    fixture.detectChanges();
    component.filePath = 'src/main.go';
    component.fetchAndAudit();
    expect(gogsServiceMock.fetchFile).not.toHaveBeenCalled();
  });

  it('should retry loading repos when retry button is clicked', () => {
    // First call fails
    gogsServiceMock.listRepos.mockReturnValueOnce(
      throwError(() => new HttpErrorResponse({ status: 500, statusText: 'Internal Server Error' })),
    );
    gogsServiceMock.listRepos.mockReturnValueOnce(of(mockRepos));

    fixture.detectChanges();
    expect(component.error()).toBeTruthy();

    // Retry
    component.loadRepos();
    expect(gogsServiceMock.listRepos).toHaveBeenCalledTimes(2);
    expect(component.error()).toBeNull();
    expect(component.repos()).toEqual(mockRepos);
  });
});