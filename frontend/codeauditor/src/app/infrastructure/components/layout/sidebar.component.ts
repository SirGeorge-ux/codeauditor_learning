import { Component, signal, inject } from '@angular/core';
import { Router, RouterModule } from '@angular/router';
import { CommonModule } from '@angular/common';
import { LucideLayoutDashboard, LucideBinary, LucideServer, LucideShield } from '@lucide/angular';

interface NavItem {
  icon: string;
  label: string;
  route: string;
}

@Component({
  selector: 'app-sidebar',
  standalone: true,
  imports: [
    CommonModule,
    RouterModule,
    LucideLayoutDashboard,
    LucideBinary,
    LucideServer,
    LucideShield,
  ],
  template: `
    <aside
      class="h-screen bg-[#0D1117] border-r border-[#21262D] flex flex-col transition-all duration-200"
      [class.w-16]="isCollapsed()"
      [class.w-56]="!isCollapsed()"
      (mouseenter)="isCollapsed.set(false)"
      (mouseleave)="isCollapsed.set(true)"
    >
      <!-- Logo area -->
      <div class="h-14 flex items-center justify-center border-b border-[#21262D]">
        @if (!isCollapsed()) {
          <span class="text-sm font-bold text-[#C9D1D9]">CodeAuditor</span>
        } @else {
          <span class="text-sm font-bold text-[#C9D1D9]">CA</span>
        }
      </div>

      <!-- Nav items -->
      <nav class="flex-1 flex flex-col gap-1 p-2">
        @for (item of navItems; track item.route) {
          <a
            [routerLink]="item.route"
            class="flex items-center gap-3 px-3 py-2.5 rounded-sm text-sm transition-colors"
            [class.text-blue-400]="activeRoute() === item.route"
            [class.text-[#8B949E]]="activeRoute() !== item.route"
            [class.hover:text-[#C9D1D9]]="activeRoute() !== item.route"
            [class.hover:bg-[#161B22]]="true"
            [class.justify-center]="isCollapsed()"
          >
            @switch (item.icon) {
              @case ('dashboard') {
                <svg lucideLayoutDashboard class="w-5 h-5"></svg>
              }
              @case ('dojo') {
                <svg lucideBinary class="w-5 h-5"></svg>
              }
              @case ('mcp') {
                <svg lucideServer class="w-5 h-5"></svg>
              }
              @case ('vault') {
                <svg lucideShield class="w-5 h-5"></svg>
              }
            }
            @if (!isCollapsed()) {
              <span>{{ item.label }}</span>
            }
          </a>
        }
      </nav>
    </aside>
  `,
})
export class SidebarComponent {
  isCollapsed = signal(true);
  activeRoute = signal('');

  navItems: NavItem[] = [
    { icon: 'dashboard', label: 'Dashboard', route: '/dashboard' },
    { icon: 'dojo', label: 'Dojo', route: '/dojo' },
    { icon: 'mcp', label: 'MCP Connections', route: '/mcp' },
    { icon: 'vault', label: 'Vault', route: '/vault' },
  ];

  constructor() {
    const router = inject(Router);
    this.activeRoute.set(router.url);
    router.events.subscribe(() => {
      this.activeRoute.set(router.url);
    });
  }
}
