import { Component, ElementRef, ViewChild, AfterViewInit, Input, OnDestroy } from '@angular/core';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

@Component({
  selector: 'app-terminal-panel',
  standalone: true,
  template: `<div #terminalContainer class="h-full w-full"></div>`,
})
export class TerminalPanelComponent implements AfterViewInit, OnDestroy {
  @ViewChild('terminalContainer', { static: true }) container!: ElementRef;

  @Input() readOnly = true;

  terminal: Terminal | null = null;
  private fitAddon: FitAddon | null = null;
  private resizeListener: (() => void) | null = null;

  ngAfterViewInit(): void {
    this.terminal = new Terminal({
      theme: {
        background: '#000000',
        foreground: '#00FF00',
        cursor: '#00FF00',
      },
      cursorBlink: true,
      disableStdin: this.readOnly,
      convertEol: true,
    });

    this.fitAddon = new FitAddon();
    this.terminal.loadAddon(this.fitAddon);
    this.terminal.open(this.container.nativeElement);
    this.fitAddon.fit();

    // Handle window resize
    this.resizeListener = () => {
      this.fitAddon?.fit();
    };
    window.addEventListener('resize', this.resizeListener);
  }

  ngOnDestroy(): void {
    if (this.resizeListener) {
      window.removeEventListener('resize', this.resizeListener);
    }
    this.terminal?.dispose();
  }

  write(data: string): void {
    this.terminal?.write(data);
  }

  writeln(data: string): void {
    this.terminal?.writeln(data);
  }

  clear(): void {
    this.terminal?.clear();
  }
}