// Domain layer — pure TypeScript with ZERO Angular imports.
// Contains entities, value objects, and domain-level interfaces.
//
// This layer must not depend on any Angular, RxJS, or infrastructure code.
// It represents the core business model of CodeAuditor.
export * from "./models";
export * from "./ports";