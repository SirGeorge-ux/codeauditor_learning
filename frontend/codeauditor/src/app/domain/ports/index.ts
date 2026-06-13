// Domain ports — interfaces for external dependencies.
// These are implemented by the infrastructure layer.
//
// Unlike Angular services, domain ports are plain TypeScript interfaces
// that express intent without coupling to any framework.
export * from "./audit-repository.port";
export * from "./auth.port";
export * from "./challenge-repository.port";
export * from "./llm.port";