import { Directive, ElementRef, HostListener, Input, output, signal, inject } from '@angular/core';

@Directive({
  selector: '[appResize]',
  standalone: true,
})
export class ResizeDirective {
  private el = inject(ElementRef<HTMLElement>);
  private startX = 0;
  private isResizing = false;

  @Input() minWidth = 200;
  @Input() maxWidth = 800;
  @Input() initialWidth = 400;

  widthChange = output<number>();

  private currentWidth = signal(400);

  constructor() {
    this.currentWidth.set(this.initialWidth);
    this.updateWidth(this.initialWidth);
  }

  private updateWidth(width: number): void {
    const clamped = Math.min(Math.max(width, this.minWidth), this.maxWidth);
    this.el.nativeElement.style.width = `${clamped}px`;
    this.currentWidth.set(clamped);
    this.widthChange.emit(clamped);
  }

  @HostListener('mousedown', ['$event'])
  onMouseDown(event: MouseEvent): void {
    this.isResizing = true;
    this.startX = event.clientX;
    event.preventDefault();
  }

  @HostListener('document:mousemove', ['$event'])
  onMouseMove(event: MouseEvent): void {
    if (!this.isResizing) return;

    const delta = event.clientX - this.startX;
    this.startX = event.clientX;
    const newWidth = this.currentWidth() + delta;
    this.updateWidth(newWidth);
  }

  @HostListener('document:mouseup')
  onMouseUp(): void {
    this.isResizing = false;
  }
}