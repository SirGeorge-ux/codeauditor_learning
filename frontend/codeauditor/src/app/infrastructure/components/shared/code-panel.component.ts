import { Component, ElementRef, ViewChild, AfterViewInit, Input, OnChanges, SimpleChanges } from '@angular/core';

// Monaco is loaded via AMD loader from the CDN in development
// eslint-disable-next-line @typescript-eslint/no-explicit-any
declare const monaco: any;

// Monaco editor instance type (loosely typed — Monaco types are optional)
type MonacoEditor = { setValue(v: string): void; getValue(): string; getModel(): { getValue(): string } };

@Component({
  selector: 'app-code-panel',
  standalone: true,
  template: `<div #editorContainer class="h-full w-full bg-[#1E1E1E]"></div>`,
})
export class CodePanelComponent implements AfterViewInit, OnChanges {
  @ViewChild('editorContainer', { static: true }) container!: ElementRef;

  @Input() code = '';
  @Input() language = 'typescript';
  @Input() readOnly = false;

  private editor: MonacoEditor | null = null;

  ngAfterViewInit(): void {
    this.initMonaco();
  }

  private initMonaco(): void {
    // Check if Monaco is already loaded (loaded via script tag in index.html)
    if (typeof monaco !== 'undefined') {
      this.createEditor();
    } else {
      // Load Monaco from CDN if not available
      const script = document.createElement('script');
      script.src = 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs/loader.js';
      script.onload = () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (window as any).require.config({
          paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' }
        });
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (window as any).require(['vs/editor/editor.main'], () => {
          this.createEditor();
        });
      };
      document.head.appendChild(script);
    }
  }

  private createEditor(): void {
    if (!this.container?.nativeElement) return;

    this.editor = monaco.editor.create(this.container.nativeElement, {
      theme: 'vs-dark',
      minimap: { enabled: false },
      fontSize: 14,
      readOnly: this.readOnly,
      value: this.code,
      automaticLayout: true,
      scrollBeyondLastLine: false,
      wordWrap: 'on',
    });
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (this.editor && changes['code'] && !changes['code'].firstChange) {
      this.setValue(this.code);
    }
  }

  setValue(value: string): void {
    if (this.editor) {
      this.editor.setValue(value);
    }
  }

  getValue(): string {
    return this.editor?.getValue() ?? '';
  }
}