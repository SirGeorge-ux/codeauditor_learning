import { Injectable, NgZone, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { AuditEvent } from '../../domain/models/audit-event';

@Injectable({ providedIn: 'root' })
export class AuditService {
  private zone = inject(NgZone);

  runAudit(code: string, language: string, challengeId: string): Observable<AuditEvent> {
    return new Observable<AuditEvent>((observer) => {
      const controller = new AbortController();

      fetch('/api/v1/audit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code, language, challengeId }),
        signal: controller.signal,
      }).then(async (response) => {
        if (!response.ok) {
          observer.error(new Error(`Server error: ${response.status}`));
          return;
        }

        const reader = response.body?.getReader();
        if (!reader) {
          observer.error(new Error('No response body'));
          return;
        }

        const decoder = new TextDecoder();
        let buffer = '';

        const readChunk = async (): Promise<void> => {
          const { done, value } = await reader.read();
          if (done) {
            // Process any remaining buffer
            if (buffer.startsWith('data: ')) {
              try {
                const parsed: AuditEvent = JSON.parse(buffer.slice(6));
                this.zone.run(() => observer.next(parsed));
              } catch { /* skip malformed */ }
            }
            this.zone.run(() => observer.complete());
            return;
          }

          buffer += decoder.decode(value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop() || '';

          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const parsed: AuditEvent = JSON.parse(line.slice(6));
                this.zone.run(() => observer.next(parsed));
                if (parsed.type === 'complete') {
                  this.zone.run(() => observer.complete());
                  return;
                }
              } catch { /* skip malformed events */ }
            }
          }

          // Continue reading
          readChunk();
        };

        readChunk();
      }).catch((err) => {
        if (err.name !== 'AbortError') {
          this.zone.run(() => observer.error(err));
        }
      });

      return () => controller.abort();
    });
  }
}