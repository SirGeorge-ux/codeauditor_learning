import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface AuditSession {
  id: string;
  user_id: string;
  challenge_title: string;
  language: string;
  code_snippet: string;
  findings_count: number;
  created_at: string;
}

export interface AuditStats {
  total_audits: number;
  total_findings: number;
}

@Injectable({ providedIn: 'root' })
export class VaultService {
  private http = inject(HttpClient);

  getHistory(): Observable<AuditSession[]> {
    return this.http.get<AuditSession[]>('/api/v1/audit/history');
  }

  getStats(): Observable<AuditStats> {
    return this.http.get<AuditStats>('/api/v1/audit/stats');
  }
}
