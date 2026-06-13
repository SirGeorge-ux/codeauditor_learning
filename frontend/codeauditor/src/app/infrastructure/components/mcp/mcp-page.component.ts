import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-mcp-page',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="p-6">
      <h1 class="text-2xl font-bold text-[#C9D1D9]">MCP Connections</h1>
    </div>
  `,
})
export class McpPageComponent {}