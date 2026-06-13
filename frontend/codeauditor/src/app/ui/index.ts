// UI layer — pure presentational components and layout.
// Components here should be as "dumb" as possible — they receive data via @Input()
// and emit events via @Output(). They delegate all logic to services.
//
// This layer DEPENDS on domain/application/infrastructure — never the reverse.
export * from "./home.component";