import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { provideHttpClient, withInterceptorsFromDi } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { GogsService, GogsRepo, GogsFileResponse } from './gogs.service';
import { environment } from '../../../environments/environment';

describe('GogsService', () => {
  let service: GogsService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        GogsService,
        provideHttpClient(withInterceptorsFromDi()),
        provideHttpClientTesting(),
      ],
    });

    service = TestBed.inject(GogsService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  describe('listRepos', () => {
    it('should call GET /api/v1/gogs/repos and return Observable<GogsRepo[]>', () => {
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
      ];

      service.listRepos().subscribe((repos) => {
        expect(repos).toEqual(mockRepos);
        expect(repos.length).toBe(1);
        expect(repos[0].full_name).toBe('org/test-repo');
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/repos`);
      expect(req.request.method).toBe('GET');
      req.flush(mockRepos);
    });

    it('should return empty array when no repos exist', () => {
      service.listRepos().subscribe((repos) => {
        expect(repos).toEqual([]);
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/repos`);
      req.flush([]);
    });

    it('should handle error response', () => {
      service.listRepos().subscribe({
        next: () => expect.unreachable('Should have errored'),
        error: (err) => {
          expect(err.status).toBe(500);
        },
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/repos`);
      req.flush('Server error', { status: 500, statusText: 'Internal Server Error' });
    });
  });

  describe('fetchFile', () => {
    it('should call POST /api/v1/gogs/file with correct body and return Observable<GogsFileResponse>', () => {
      const mockResponse: GogsFileResponse = {
        owner: 'org',
        repo: 'test-repo',
        branch: 'main',
        path: 'src/main.go',
        content: btoa('package main\n\nfunc main() {}'),
        encoding: 'base64',
        language: 'go',
        size: 30,
      };

      service.fetchFile('org', 'test-repo', 'main', 'src/main.go').subscribe((response) => {
        expect(response).toEqual(mockResponse);
        expect(response.language).toBe('go');
        expect(response.encoding).toBe('base64');
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/file`);
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({
        owner: 'org',
        repo: 'test-repo',
        branch: 'main',
        path: 'src/main.go',
      });
      req.flush(mockResponse);
    });

    it('should handle 404 file not found', () => {
      service.fetchFile('org', 'test-repo', 'main', 'nonexistent.go').subscribe({
        next: () => expect.unreachable('Should have errored'),
        error: (err) => {
          expect(err.status).toBe(404);
        },
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/file`);
      req.flush('File not found', { status: 404, statusText: 'Not Found' });
    });

    it('should handle file too large error (413)', () => {
      service.fetchFile('org', 'big-repo', 'main', 'large-file.go').subscribe({
        next: () => expect.unreachable('Should have errored'),
        error: (err) => {
          expect(err.status).toBe(413);
        },
      });

      const req = httpMock.expectOne(`${environment.apiUrl}/api/v1/gogs/file`);
      req.flush('FILE_TOO_LARGE', { status: 413, statusText: 'Payload Too Large' });
    });
  });
});