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
import { ProfilePageComponent } from './infrastructure/components/profile/profile-page.component';
import { DictionaryPageComponent } from './infrastructure/components/dictionary/dictionary-page.component';
import { DictionaryDetailComponent } from './infrastructure/components/dictionary/dictionary-detail.component';
import { PracticePageComponent } from './infrastructure/components/practice/practice-page.component';

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
    canActivate: [authGuard],
    children: [
      { path: 'dashboard', component: DashboardPageComponent },
      { path: 'dojo', component: DojoPageComponent },
      { path: 'dojo/:id', component: DojoPageComponent },
      // Practice mode: S3 spec landed. The canonical entry point is
      // `/practice/free` (per design.md §Data Shapes + the proposal's
      // intended route). `/practice` and `/practice/:language` are
      // preserved as aliases so existing links (e.g. from the tutor
      // chat's /diccionario command) keep working without a migration.
      { path: 'practice', redirectTo: 'practice/free', pathMatch: 'full' },
      { path: 'practice/free', component: PracticePageComponent },
      { path: 'practice/:language', component: PracticePageComponent },
      { path: 'mcp', component: McpPageComponent },
      { path: 'vault', component: VaultPageComponent },
      { path: 'profile', component: ProfilePageComponent },
      { path: 'dictionary', component: DictionaryPageComponent },
      { path: 'dictionary/:id', component: DictionaryDetailComponent },
    ],
  },
  {
    path: '**',
    redirectTo: '/dashboard',
  },
];
