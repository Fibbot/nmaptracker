# code_audit.md

## Purpose
Perform a thorough security review of an application codebase that processes and stores PII. Produce actionable findings and concrete fixes required for a production-ready security posture.

## Scope
- Application code (frontend, backend, workers, CLI, scripts)
- Infrastructure-as-code, deployment manifests, CI/CD, container builds
- Dependencies and supply chain
- Authentication/authorization flows
- Data storage and data movement (PII lifecycle)
- Logging/monitoring/telemetry
- Secrets and configuration

## Output Requirements
Deliverables must include:
1. **Executive Summary**: current risk posture, highest-risk issues, go/no-go recommendation for production.
2. **Findings List**: each finding includes:
   - ID, Title, Severity (Critical/High/Medium/Low), Likelihood, Impact
   - Affected components/files/functions/endpoints
   - Exploit scenario (step-by-step, minimal)
   - Evidence (code references)
   - Remediation (exact changes, patterns, or patches)
   - Verification steps (how to confirm fix)
3. **PII Map**: where PII is collected, validated, stored, transmitted, logged, exported, deleted.
4. **Threat Model Snapshot**: key trust boundaries, attacker types, top abuse cases.
5. **Security Checklist**: pass/fail with notes.

## Severity Guidance
- **Critical**: direct account takeover, auth bypass, RCE, SQLi, mass PII exposure, secrets exfiltration path.
- **High**: privilege escalation, IDOR exposing sensitive data, SSRF w/ metadata access, persistent XSS, weak crypto exposing PII.
- **Medium**: limited data exposure, DoS feasible, insecure defaults, missing rate limiting on sensitive routes.
- **Low**: hardening, best-practice gaps without clear exploit path.

---

## Phase 0: Baseline Understanding
### 0.1 Inventory & Architecture
- Identify runtimes/languages/frameworks.
- Enumerate entry points: HTTP APIs, GraphQL, RPC, WebSockets, background jobs, file upload, admin panels.
- Map data stores: SQL/NoSQL, caches, queues, object storage, search indexes.
- Identify external integrations: auth providers, payment, email/SMS, analytics, webhooks.
- Identify deployment model: containers, serverless, VM, k8s.

### 0.2 Trust Boundaries
Document boundaries such as:
- Internet ↔ edge (CDN/WAF) ↔ app
- app ↔ database/cache/queue/object storage
- app ↔ third parties
- internal admin ↔ public user
- multi-tenant boundaries (tenant isolation)

---

## Phase 1: PII & Privacy Controls (Must Be Thorough)
### 1.1 PII Identification & Classification
- Enumerate all PII fields (names, emails, addresses, phone, DOB, IDs, tokens, IPs, device IDs).
- Classify: sensitive PII vs general PII; mark regulated categories if present.
- Identify derived data (profiles, risk scores, logs containing identifiers).

### 1.2 PII Lifecycle Checks
For each PII field/path:
- **Collection**: consent, minimization, validation, front-end/back-end consistency.
- **Transmission**: TLS enforced, no downgrade, no mixed content, secure cookies.
- **Storage**:
  - encryption at rest (platform-managed or app-managed) where appropriate
  - field-level encryption for highly sensitive attributes if needed
  - key management (KMS/HSM), rotation, separation of duties
- **Access**:
  - strict authorization checks
  - least privilege for services and humans
  - audit trails for access to PII
- **Logging**:
  - ensure PII is not logged (request/response bodies, headers, query params)
  - redact/transform identifiers (hashing with salt where appropriate)
- **Retention/Deletion**:
  - retention windows and deletion workflows exist and are enforced
  - backups and replicas handling documented
- **Export**:
  - data export endpoints protected, rate limited, and audited
  - prevent mass-export and scraping

### 1.3 Data Minimization and Exposure
- Ensure responses only include necessary fields (no overfetch).
- Default to “private by default” in serializers/DTOs.
- Pagination required for list endpoints returning user data.
- Search endpoints must not leak existence of accounts via timing/messages.

---

## Phase 2: Authentication & Session Security
### 2.1 Auth Mechanisms
- Identify auth types: password, magic link, OAuth/OIDC, SAML, API keys, JWT sessions.
- Confirm:
  - strong password policy if passwords exist
  - secure password hashing (Argon2id/bcrypt with proper params)
  - MFA support for admin/high-risk operations (required for production where feasible)
  - magic links are single-use, short-lived, bound to device/session where possible
  - OAuth: correct redirect URI validation, state/nonce usage

### 2.2 Session Management
- Cookies: `HttpOnly`, `Secure`, `SameSite` (Lax/Strict as appropriate)
- Session rotation on privilege change and login
- Logout invalidates server-side sessions / revokes tokens where applicable
- Token storage: avoid localStorage for high-value tokens unless unavoidable and justified
- CSRF protection for cookie-based sessions
- Brute-force defenses: rate limits, lockouts, device fingerprinting (careful with privacy)

### 2.3 Account Recovery & Registration
- Prevent account enumeration (uniform responses)
- Rate limit password reset / magic link / verification
- Ensure recovery tokens are random, one-time, short-lived
- Validate email/phone ownership properly
- Admin account bootstrap process is secure and auditable

---

## Phase 3: Authorization (Most Common Real-World Failures)
### 3.1 Access Control Model
- Identify RBAC/ABAC/ACL rules and enforcement points.
- Verify authorization is enforced server-side for every:
  - read/write of user resources
  - tenant-scoped access
  - admin/impersonation features
  - file/object retrieval

### 3.2 IDOR / Multi-Tenant Isolation
- Ensure object ownership checks on every resource by ID.
- Avoid trusting user-supplied tenant IDs; derive from session.
- Validate row-level security if used (verify policies).
- Ensure list endpoints filter by tenant/user.

### 3.3 Privileged Actions
- Extra controls for:
  - role changes
  - billing changes
  - exporting data
  - webhook configuration
  - API key creation
- Require re-authentication for sensitive operations where feasible.

---

## Phase 4: Input Handling & Common Web Vulnerabilities
### 4.1 Input Validation & Normalization
- Validate all inbound data at the boundary:
  - types, length, formats, allowed characters
  - canonicalization (unicode normalization)
- Server-side validation must not rely on client.
- Centralize schemas (e.g., zod/joi/pydantic) and reuse across endpoints.

### 4.2 Injection Classes
Check for:
- SQL/NoSQL injection (string concatenation, unsafe query builders)
- Command injection (shell calls, process execution)
- Template injection (server templates, expression languages)
- LDAP injection (if applicable)
- SSRF (URL fetchers, image proxy, webhook testing)
- Deserialization issues (unsafe deserialization of untrusted bytes)

### 4.3 XSS & Content Injection
- Stored/reflected XSS in any HTML rendering
- React/Vue unsafe rendering (`dangerouslySetInnerHTML`)
- Markdown rendering sanitization
- CSP presence and correctness (report-only vs enforced)
- Output encoding based on context (HTML, JS, URL, CSS)

### 4.4 CSRF / Clickjacking
- CSRF tokens for state-changing requests if cookie-based auth
- Ensure idempotency and correct HTTP verbs
- `X-Frame-Options` / CSP `frame-ancestors`

### 4.5 File Uploads
- Content-type validation + magic-byte sniffing
- Size limits, file count limits
- Store outside web root, random names, no user-controlled paths
- Virus scanning where appropriate
- Prevent SSRF via file parsing (e.g., PDF/image libraries)
- Ensure direct object access is authorized (signed URLs scoped tightly)

---

## Phase 5: Cryptography & Secrets
### 5.1 Crypto Usage
- No custom crypto
- Approved algorithms and modes:
  - AES-GCM / ChaCha20-Poly1305 for encryption
  - HMAC-SHA256/512 for integrity
- Unique nonces/IVs, proper randomness source
- Key rotation strategy
- Separate keys for separate purposes (encryption vs signing)

### 5.2 Secrets Management
- No secrets in source control (including sample .env with real creds)
- CI/CD secrets stored in secret manager
- Runtime secrets injected securely
- Rotate leaked keys; document incident response
- Prevent secrets in logs, error pages, client bundles

### 5.3 JWT / Token Handling
- Validate signature, issuer, audience, expiry
- Reject `none` alg and unexpected algs
- Verify key rotation / JWKS caching safely
- Avoid embedding PII in tokens unless required; treat as sensitive if present

---

## Phase 6: Dependency & Supply Chain Security
### 6.1 Dependency Review
- Enable lockfiles and integrity checks
- Identify high-risk deps: crypto, auth, parsers, upload libs, template engines
- Check for:
  - known CVEs
  - unmaintained packages
  - suspicious maintainer changes / typosquatting

### 6.2 Build & CI/CD Hardening
- Pin actions and base images by digest
- Principle of least privilege for CI tokens
- Prevent PRs from forks accessing secrets
- Artifact signing / provenance if feasible
- SAST/dep scanning gates required for production

### 6.3 Container & Runtime
- Minimal base images, non-root user
- Read-only filesystem where possible
- Drop capabilities, seccomp/apparmor profiles if in k8s
- No debug endpoints enabled in prod

---

## Phase 7: Error Handling, Logging, Monitoring
### 7.1 Error Hygiene
- No stack traces or internal details to clients
- Uniform error responses (no enumeration)
- Safe handling of 404 vs 403 semantics where appropriate

### 7.2 Logging Controls
- Structured logs with redaction
- Do not log: auth headers, cookies, tokens, request bodies with PII
- Audit logs for PII access and privileged actions
- Log integrity and retention policy

### 7.3 Monitoring & Alerting
- Alerts for:
  - auth anomalies (bruteforce, token misuse)
  - high-rate 4xx/5xx spikes
  - unusual export volume
  - admin actions
- Ensure monitoring data does not exfiltrate PII to third-party analytics

---

## Phase 8: Business Logic & Abuse Cases
- Identify abuse vectors:
  - free-tier abuse
  - scraping
  - invitation hijacking
  - promo/referral fraud
  - webhook replay/forgery
- Validate:
  - idempotency keys for payments/critical writes
  - replay protection for webhooks (signature + timestamp)
  - rate limiting per user/IP/tenant
  - captcha/turnstile where appropriate (careful with UX)

---

## Phase 9: Configuration & Environment Review
- Secure defaults:
  - prod disables debug, verbose logging, dev CORS
- CORS:
  - no wildcard with credentials
  - explicit origins for prod
- Security headers:
  - HSTS, CSP, X-Content-Type-Options, Referrer-Policy, Permissions-Policy
- TLS:
  - modern cipher suites, TLS 1.2+ (prefer 1.3)
- Verify environment separation (dev/stage/prod):
  - no shared secrets
  - no shared databases for PII

---

## Testing & Verification Playbook
### Required Code Review Tactics
- Trace auth from edge → handler → service → datastore
- Grep patterns:
  - `SELECT .* ${` or string interpolation into queries
  - `exec(` / `spawn(` / `system(` / `shell=True`
  - `dangerouslySetInnerHTML` / raw HTML renderers
  - `http://` usage, `verify=False`, insecure TLS settings
  - `eval`, dynamic requires/imports, reflection on user input
- Review middleware ordering (auth before handlers; body parsing limits)
- Ensure consistent authorization helpers are used everywhere.

### Required Dynamic Checks (If runnable locally)
- Attempt IDOR by swapping resource IDs across tenants
- Check rate limits on login/reset/export endpoints
- Validate CSRF if cookie-based
- Upload malicious files (polyglots, oversized, mime spoof)
- Probe SSRF endpoints with internal IP ranges and metadata URLs

---

## Production Readiness Security Gate
Application is **not production-ready** if any of the following are true:
- Any unauthenticated access to PII
- Any tenant isolation bypass / IDOR exposing data
- Any known exploitable injection (SQLi/command/SSRF to metadata)
- Secrets in repo or client bundle
- Missing authorization checks on any PII-accessing endpoint
- No audit logging for privileged/PII access actions

## Final Report Template
- Executive summary (1 page)
- Risk register (table)
- PII map diagram/list
- Findings (detailed)
- Remediation plan by priority (0–7 days, 7–30 days, 30–90 days)
- Verification checklist
