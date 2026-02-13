# optimization_agent.md

## Purpose
Optimize application performance for production: latency, throughput, responsiveness, and resource efficiency. Focus on end-user experience (page load time, TTFB, API latency) and backend scalability while maintaining correctness and security.

## Scope
- Frontend performance (bundles, rendering, hydration, network)
- Backend performance (DB queries, caching, concurrency)
- Infrastructure/runtime (containers, autoscaling, CDN)
- Observability for performance (metrics, traces, profiling)
- Efficiency patterns (avoid wasted work; reuse; batch; stream)

## Output Requirements
Deliverables must include:
1. **Performance Baseline**: current p50/p95/p99 for key endpoints/pages, key bottlenecks.
2. **Top Opportunities**: ranked list by impact/effort.
3. **Change Plan**: concrete changes with files/areas to modify and expected impact.
4. **Validation**: how to measure improvements and prevent regressions.

## Key Metrics (Track These)
- Frontend: LCP, INP, CLS, TTFB, JS bundle size, number of requests
- Backend: request latency p95/p99, DB query time, cache hit rate, queue lag
- Infra: CPU, memory, GC pauses, connection pool saturation, error rates
- Network: payload sizes, compression ratios, CDN hit rate

---

## Phase 0: Establish Measurement (No Optimizing Blind)
### 0.1 Instrumentation
- Ensure distributed tracing exists (or add it) for:
  - request → controller → service → DB/cache/external calls
- Add per-endpoint timing + slow query logs (with sampling).
- Add profiling hooks appropriate for the runtime (CPU/memory/heap).

### 0.2 Define SLOs
- Set targets (example):
  - API p95 < 200–400ms for common calls
  - page navigation < 1–2s on broadband
  - background jobs within defined SLA
- Define “critical user journeys” to measure.

---

## Phase 1: Frontend Performance
### 1.1 Bundle & Asset Optimization
- Remove unused deps; tree-shake; verify production build flags.
- Code split routes; lazy-load non-critical components.
- Reduce bundle size:
  - replace heavy libs with lighter alternatives where safe
  - avoid shipping server-only code
- Images:
  - responsive sizes, modern formats, proper caching headers
  - lazy-load below the fold
- Fonts:
  - subset, preload critical, avoid blocking rendering

### 1.2 Rendering & UX Responsiveness
- Avoid unnecessary re-renders; memoize expensive components.
- Virtualize long lists.
- Defer non-critical work (analytics, tooltips, editors).
- Use streaming/partial rendering if framework supports it.
- Reduce hydration cost; minimize client-side JS where possible.

### 1.3 Network Efficiency
- HTTP caching headers for static assets
- Use CDN for static and media
- Enable compression (brotli/gzip) for text responses
- Batch requests or use query batching where appropriate
- Avoid overfetch (trim JSON payloads; field selection)

---

## Phase 2: Backend Performance
### 2.1 Endpoint Profiling
For top endpoints by traffic or latency:
- Break down time spent:
  - parsing/validation
  - authn/authz
  - DB time
  - cache
  - external services
  - serialization
- Identify N+1 patterns.

### 2.2 Database Efficiency
- Query optimization:
  - add/adjust indexes based on query plans
  - eliminate N+1; use joins or batch selects
  - select only needed columns
- Connection pooling tuned to runtime and DB limits
- Transactions:
  - keep short; avoid long locks
  - correct isolation where needed
- Pagination everywhere for lists; avoid OFFSET for huge tables (prefer keyset pagination)

### 2.3 Caching Strategy
- Decide what to cache:
  - read-heavy endpoints, expensive computed views, feature flags
- Cache layers:
  - CDN/edge for public-ish responses
  - server cache (in-memory) for hot configs
  - distributed cache (redis/memcached) for shared hot data
- Correctness controls:
  - TTLs, cache busting, versioned keys
  - avoid caching personalized/PII responses unless scoped per-user and safe
- Measure hit rate and stale/miss penalties.

### 2.4 Concurrency & Async Work
- Move slow operations off request path:
  - emails, webhooks, reports, large exports
- Use queues and background workers.
- Ensure timeouts and retries with backoff for external calls.
- Avoid thundering herd:
  - request coalescing, singleflight, locks on cache fill

### 2.5 Serialization & Payload Size
- Trim response objects; avoid sending redundant data.
- Use streaming for large exports.
- Prefer binary formats only if ecosystem supports and benefits are clear.

---

## Phase 3: Infrastructure & Runtime Efficiency
### 3.1 Runtime Tuning
- Set appropriate timeouts: request, upstream, DB, external.
- Tune GC/heap settings where applicable.
- Ensure worker counts match CPU and workload type.

### 3.2 Deployment Optimizations
- Use autoscaling based on meaningful signals (CPU + latency + queue depth).
- Ensure cold start issues are addressed (warm pools, keep-alive).
- Use HTTP keep-alive and connection reuse.

### 3.3 CDN & Edge
- Cache static assets aggressively with immutable hashes.
- Consider edge caching for non-personalized GETs.
- Reduce origin hits; validate cache-control headers.

---

## Phase 4: “Efficient Patterns” Checklist (Quality of Implementation)
Use these patterns broadly:
- Avoid repeated work: compute once, reuse (memoization, caching, shared loaders)
- Batch: DB calls, external API calls, writes
- Stream: large responses instead of buffering
- Bound work: rate limits, backpressure, queue limits
- Fail fast: validate early, short-circuit on errors
- Prefer O(1)/O(log n) approaches for hot paths
- Precompute: derived aggregates updated incrementally instead of recomputed per request
- Use proper data structures (maps/sets) for membership checks in hot loops

---

## Phase 5: Regression Prevention
### 5.1 Performance Budgets
- Set budgets:
  - max JS bundle size
  - max number of requests on initial load
  - max p95 for key endpoints
- Add CI checks for bundle size diffs and lighthouse/web vitals (where feasible).

### 5.2 Load Testing
- Create representative load tests for:
  - login
  - key page loads
  - top API endpoints
  - search/list endpoints
- Track p95/p99 and error rate under load.

### 5.3 Profiling in CI/Staging
- Periodic profiling runs to catch regressions.
- Alerting on latency regressions over baseline.

---

## Optimization Execution Order (High ROI First)
1. Measure + identify top 5 slow endpoints/pages.
2. Fix N+1 queries and missing indexes.
3. Reduce payload sizes and enable proper caching.
4. Frontend code splitting and asset optimization.
5. Move slow side effects to async jobs.
6. Tune pools/timeouts and add backpressure.
7. Add performance budgets and regression gates.

## Final Report Template
- Baseline metrics (before)
- Bottleneck analysis (traces/flamegraphs summaries)
- Change list with expected impact
- Post-change metrics (after)
- Remaining backlog
- Regression gates added
