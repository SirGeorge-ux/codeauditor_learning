import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { ProfilePageComponent } from './profile-page.component';
import { AuthService } from '../../services/auth.service';
import { ProgressService } from '../../services/progress.service';
import { AllProgressResponse } from '../../../domain/models/language-progress';
import { LearningProfile } from '../../../domain/models/learning-profile';

describe('ProfilePageComponent', () => {
  let component: ProfilePageComponent;
  let fixture: ComponentFixture<ProfilePageComponent>;
  let progressServiceMock: {
    languages: ReturnType<typeof vi.fn>;
    rangoGlobal: ReturnType<typeof vi.fn>;
    learningProfile: ReturnType<typeof vi.fn>;
    loading: ReturnType<typeof vi.fn>;
    error: ReturnType<typeof vi.fn>;
    loadProgress: ReturnType<typeof vi.fn>;
    loadLearningProfile: ReturnType<typeof vi.fn>;
    saveLearningProfile: ReturnType<typeof vi.fn>;
    updatePreferencias: ReturnType<typeof vi.fn>;
    updateEstilo: ReturnType<typeof vi.fn>;
  };
  let authServiceMock: {
    userSignal: ReturnType<typeof vi.fn>;
  };

  const mockUserProfile = {
    id: 'user-uuid-123',
    email: 'test@example.com',
    display_name: 'TestUser',
    racha_dias: 5,
    puntos_maestria: 1200,
    rango_actual: 'Mid',
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-02T00:00:00Z',
  };

  const mockLanguages: AllProgressResponse = {
    rangoGlobal: 'D',
    languages: [
      {
        userId: 'user-uuid-123',
        language: 'typescript',
        rango: 'D',
        puntos: 300,
        challengesCompletados: 5,
        challengesIntentados: 8,
        tasaExito: 62.5,
        topicsDominados: ['async', 'generics'],
        rangoCodeHealth: 'Junior',
        puntosCodeHealth: 50,
        reposAnalizados: 2,
        issuesEncontrados: 10,
        ultimoCompletado: '2025-07-20T00:00:00Z',
        ultimaActualizacion: '2025-07-20T00:00:00Z',
      },
      {
        userId: 'user-uuid-123',
        language: 'go',
        rango: 'F',
        puntos: 10,
        challengesCompletados: 1,
        challengesIntentados: 3,
        tasaExito: 33.3,
        topicsDominados: [],
        rangoCodeHealth: 'Junior',
        puntosCodeHealth: 0,
        reposAnalizados: 0,
        issuesEncontrados: 0,
        ultimoCompletado: null,
        ultimaActualizacion: '2025-07-18T00:00:00Z',
      },
    ],
  };

  const mockLearningProfile: LearningProfile = {
    userId: 'user-uuid-123',
    preferencias: {
      idioma: 'es',
      nivelSocratismo: 2,
      longitudMaximaMsg: 200,
      bloqueosTipicos: [],
      topicsQueLeCuestan: [],
      resumen: '',
    },
    estiloAprendizaje: {
      aprendeMejorCon: 'ejemplos',
    },
    ultimaActualizacion: '2025-07-22T00:00:00Z',
  };

  beforeEach(async () => {
    progressServiceMock = {
      languages: vi.fn(() => mockLanguages.languages),
      rangoGlobal: vi.fn(() => mockLanguages.rangoGlobal),
      learningProfile: vi.fn(() => mockLearningProfile),
      loading: vi.fn(() => false),
      error: vi.fn(() => null),
      loadProgress: vi.fn().mockResolvedValue(undefined),
      loadLearningProfile: vi.fn().mockResolvedValue(undefined),
      saveLearningProfile: vi.fn().mockResolvedValue(undefined),
      updatePreferencias: vi.fn(),
      updateEstilo: vi.fn(),
    };

    authServiceMock = {
      userSignal: vi.fn(() => mockUserProfile),
    };

    await TestBed.configureTestingModule({
      imports: [ProfilePageComponent],
      providers: [
        { provide: AuthService, useValue: authServiceMock },
        { provide: ProgressService, useValue: progressServiceMock },
        provideRouter([]),
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(ProfilePageComponent);
    component = fixture.componentInstance;
  });

  it('should create the component', () => {
    fixture.detectChanges();
    expect(component).toBeTruthy();
  });

  it('should load progress and learning profile on init', () => {
    fixture.detectChanges(); // triggers ngOnInit
    expect(progressServiceMock.loadProgress).toHaveBeenCalledOnce();
    expect(progressServiceMock.loadLearningProfile).toHaveBeenCalledOnce();
  });

  it('should display user info and global rank', () => {
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('TestUser');
    expect(compiled.textContent).toContain('test@example.com');
    expect(compiled.textContent).toContain('D');
    expect(compiled.textContent).toContain('Global Rank');
  });

  it('should render language cards with rango badges', () => {
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    // Both languages should be rendered
    expect(compiled.textContent).toContain('typescript');
    expect(compiled.textContent).toContain('go');
    // F-S badges
    expect(compiled.textContent).toContain('D');
    expect(compiled.textContent).toContain('F');
    // Jr-Arch badges
    expect(compiled.textContent).toContain('Junior');
    // Points
    expect(compiled.textContent).toContain('300');
    expect(compiled.textContent).toContain('10');
  });

  it('should show topics dominados as tags', () => {
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('async');
    expect(compiled.textContent).toContain('generics');
  });

  it('should show learning profile section with socratismo level', () => {
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Learning Preferences');
    expect(compiled.textContent).toContain('Socratica'); // level 2 label
    expect(compiled.textContent).toContain('Ejemplos'); // display text for aprendeMejorCon=ejemplos
  });

  it('should show empty state when no languages', () => {
    progressServiceMock.languages = vi.fn(() => []);
    progressServiceMock.rangoGlobal = vi.fn(() => 'F');
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('No practice data yet');
    expect(compiled.textContent).toContain('Complete your first challenge');
  });

  it('should show loading state', () => {
    progressServiceMock.loading = vi.fn(() => true);
    fixture.detectChanges();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Loading progress');
  });

  it('should call saveLearningProfile on save button click', async () => {
    fixture.detectChanges();
    component.saveProfile();
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(progressServiceMock.saveLearningProfile).toHaveBeenCalledOnce();
  });

  it('should update preferencias when idioma changes', () => {
    fixture.detectChanges();
    const mockEvent = {
      target: { value: 'en' },
    } as unknown as Event;
    component.onIdiomaChange(mockEvent);
    expect(progressServiceMock.updatePreferencias).toHaveBeenCalledWith(
      expect.objectContaining({ idioma: 'en' }),
    );
  });

  it('should update preferencias when socratismo slider changes', () => {
    fixture.detectChanges();
    const mockEvent = {
      target: { value: '3' },
    } as unknown as Event;
    component.onSocratismoChange(mockEvent);
    expect(progressServiceMock.updatePreferencias).toHaveBeenCalledWith(
      expect.objectContaining({ nivelSocratismo: 3 }),
    );
  });

  it('should map socratismo levels to labels correctly', () => {
    fixture.detectChanges();
    expect(component.socratismoLabel(0)).toBe('Directa');
    expect(component.socratismoLabel(1)).toBe('Guiada');
    expect(component.socratismoLabel(2)).toBe('Socratica');
    expect(component.socratismoLabel(3)).toBe('Descubrimiento');
  });

  it('should map F-S rango to correct badge colors', () => {
    fixture.detectChanges();
    expect(component.fBadgeColor('F')).toContain('gray');
    expect(component.fBadgeColor('S')).toContain('yellow');
    expect(component.fBadgeColor('A')).toContain('purple');
  });

  it('should map Jr-Arch rango to correct badge colors', () => {
    fixture.detectChanges();
    expect(component.jrArchBadgeColor('Junior')).toContain('gray');
    expect(component.jrArchBadgeColor('Architect')).toContain('purple');
  });
});