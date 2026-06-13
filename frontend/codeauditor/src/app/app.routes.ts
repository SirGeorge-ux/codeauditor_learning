import { Routes } from '@angular/router';
import { HomeComponent } from './ui/home.component';
import { LoginComponent } from './infrastructure/components/login.component';
import { RegisterComponent } from './infrastructure/components/register.component';
import { authGuard } from './infrastructure/guards/auth.guard';
import { MainLayoutComponent } from './infrastructure/components/layout/main-layout.component';
import { DashboardPageComponent } from './infrastructure/components/dashboard/dashboard-page.component';
import { DojoPageComponent } from './infrastructure/components/dojo/dojo-page.component';
import { McpPageComponent } from './infrastructure/components/mcp/mcp-page.component';
import { VaultPageComponent } from './infrastructure/components/vault/vault-page.component';

export const routes: Routes = [
  {
    path: '',
    component: HomeComponent,
  },
  {
    path: 'login',
    component: LoginComponent,
  },
  {
    path: 'register',
    component: RegisterComponent,
  },
  {
    path: '',
    component: MainLayoutComponent,
    // canActivate: [authGuard], // ← reactivar cuando el proyecto esté completo
    children: [
      { path: 'dashboard', component: DashboardPageComponent },
      { path: 'dojo', component: DojoPageComponent },
      { path: 'dojo/:id', component: DojoPageComponent },
      { path: 'mcp', component: McpPageComponent },
      { path: 'vault', component: VaultPageComponent },
    ],
  },
  {
    path: '**',
    redirectTo: '/dashboard',
  },
];