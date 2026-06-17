// GogsService — Angular injectable service wrapping Gogs backend API calls.
//
// Provides listRepos() and fetchFile() methods using HttpClient.
// All endpoints are under JWT auth middleware on the backend.
import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';

export interface GogsRepo {
  id: number;
  name: string;
  full_name: string;
  description: string;
  private: boolean;
  clone_url: string;
  default_branch: string;
}

export interface GogsFileResponse {
  owner: string;
  repo: string;
  branch: string;
  path: string;
  content: string; // base64-encoded
  encoding: string; // "base64"
  language: string;
  size: number;
}

@Injectable({ providedIn: 'root' })
export class GogsService {
  private http = inject(HttpClient);
  private apiUrl = environment.apiUrl;

  listRepos(): Observable<GogsRepo[]> {
    return this.http.get<GogsRepo[]>(`${this.apiUrl}/api/v1/gogs/repos`);
  }

  fetchFile(
    owner: string,
    repo: string,
    branch: string,
    path: string,
  ): Observable<GogsFileResponse> {
    return this.http.post<GogsFileResponse>(`${this.apiUrl}/api/v1/gogs/file`, {
      owner,
      repo,
      branch,
      path,
    });
  }
}
