# Specification: Supabase Authentication

This specification defines the requirements and scenarios for implementing end-to-end user authentication using Supabase in the CodeAuditor project.

## ADDED Requirements

### Requirement: User Registration

The system MUST allow users to register a new account using an email and password.

#### Scenario: Successful Registration
- GIVEN an unauthenticated user on the registration UI
- WHEN the user submits a valid email and password
- THEN the system MUST create a new user account in Supabase
- AND the system MUST return a success response

#### Scenario: Registration with Existing Email
- GIVEN an unauthenticated user
- WHEN the user submits an email that is already registered
- THEN the system MUST return a conflict or validation error
- AND the system MUST NOT create a duplicate account

### Requirement: User Login

The system MUST allow users to authenticate using their email and password.

#### Scenario: Successful Login
- GIVEN a registered user on the login UI
- WHEN the user submits correct credentials
- THEN the system MUST authenticate the user with Supabase Auth
- AND the system MUST return a valid JWT session token

#### Scenario: Invalid Credentials
- GIVEN a registered user
- WHEN the user submits incorrect credentials
- THEN the system MUST return a 401 authentication error
- AND the system MUST NOT issue a JWT session token

### Requirement: JWT Validation Middleware

The system MUST validate the JWT on all protected backend routes using the `AuthValidator` port.

#### Scenario: Valid JWT Token
- GIVEN a request to a protected backend route
- WHEN the request includes a valid Supabase JWT in the Authorization header
- THEN the validation middleware MUST verify the token signature
- AND the system MUST extract the user identity and proceed to the handler

#### Scenario: Missing or Invalid JWT Token
- GIVEN a request to a protected backend route
- WHEN the request lacks a valid JWT or the token is expired/invalid
- THEN the validation middleware MUST reject the request with a 401 Unauthorized error

### Requirement: Angular Auth Service

The system MUST manage reactive session state in the frontend using an Angular `AuthService` built on signals.

#### Scenario: Session State Initialization
- GIVEN a user logs in successfully
- WHEN the JWT session is received by the frontend
- THEN the `AuthService` MUST update its signal state to reflect the authenticated user identity

#### Scenario: Session State Clearing
- GIVEN an authenticated user
- WHEN the user logs out
- THEN the `AuthService` MUST clear its signal state to reflect an unauthenticated user

### Requirement: Route Guards

The system MUST restrict access to protected frontend views based on the current authentication state.

#### Scenario: Accessing Protected Route
- GIVEN an authenticated user
- WHEN the user navigates to a protected Angular route
- THEN the route guard MUST allow access to the view

#### Scenario: Accessing Protected Route when Unauthenticated
- GIVEN an unauthenticated user
- WHEN the user attempts to navigate to a protected Angular route
- THEN the route guard MUST block access
- AND the system MUST redirect the user to the login view

### Requirement: User Logout

The system MUST allow authenticated users to securely end their session.

#### Scenario: Successful Logout
- GIVEN an authenticated user
- WHEN the user triggers the logout action
- THEN the system MUST invalidate the session via the backend logout handler
- AND the Angular `AuthService` MUST clear the local session state
