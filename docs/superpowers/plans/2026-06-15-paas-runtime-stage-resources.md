# PaaS Runtime Stage Resources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a PaaS-native ArgoCD-like runtime view from Stage cards, including Argo CD status, Kubernetes resource details, restart actions, pod logs, and gated terminal entry.

**Architecture:** Keep PaaS Web Console as the only user entrypoint. Runtime data flows from Argo CD/Kubernetes to `paas-agent`, then to the control plane, and finally to Web Console APIs; the control plane never calls Kubernetes directly. Mutating runtime operations are modeled as audited cluster tasks and executed by the in-cluster agent.

**Tech Stack:** Go, MySQL migrations/repositories, `client-go`, React, TypeScript, Ant Design, Vitest.

---

### Task 0: Baseline Cleanup

**Files:**
- Modify: `internal/modules/appenv/repository_mysql.go`
- Test: `internal/modules/appenv/service_test.go`

- [x] **Step 1: Reproduce existing baseline failure**

Run: `go test -count=1 ./internal/modules/appenv -run TestHandlerApplicationEnvironmentFlow -v`

Expected before fix: FAIL with `delete application data failed`.

- [x] **Step 2: Fix application deletion FK ordering**

Delete `workload_default_configs` before deleting `workloads` in `DeleteApplicationData`.

- [x] **Step 3: Verify focused test**

Run: `go test -count=1 ./internal/modules/appenv -run TestHandlerApplicationEnvironmentFlow -v`

Expected: PASS.

- [x] **Step 4: Verify full baseline**

Run: `scripts/test-full.sh`

Expected: Go packages pass, Vitest reports `85 passed`, frontend build succeeds.

### Task 1: Stage Runtime Status

**Files:**
- Modify: `internal/modules/delivery/domain.go`
- Modify: `internal/modules/delivery/ports.go`
- Modify: `internal/modules/delivery/service.go`
- Modify: `internal/modules/delivery/service_test.go`
- Modify: `web/console/src/pages/PromotionPage.tsx`
- Modify: `web/console/src/pages/PromotionPage.test.tsx`

- [x] **Step 1: Write failing delivery service test**

Add a test proving `ListAppStages` includes latest runtime `sync_status`, `health_status`, `operation_state`, and `runtime_message` for the stage environment.

- [x] **Step 2: Verify RED**

Run: `go test -count=1 ./internal/modules/delivery -run TestListAppStagesIncludesRuntimeStatus -v`

Expected: FAIL because runtime fields are empty or missing.

- [x] **Step 3: Implement minimal query path**

Add an environment state query port to delivery, map existing environment state into stage runtime fields, and keep values empty when no runtime state exists.

- [x] **Step 4: Verify GREEN**

Run: `go test -count=1 ./internal/modules/delivery -run TestListAppStagesIncludesRuntimeStatus -v`

Expected: PASS.

- [x] **Step 5: Add UI test**

Add a Promotion page test that Stage cards show Chinese labels for Argo CD sync/health and the latest runtime message.

- [x] **Step 6: Implement Stage card UI**

Render runtime tags on Stage cards without changing drag/drop behavior.

- [x] **Step 7: Verify UI**

Run: `cd web/console && npm test -- src/pages/PromotionPage.test.tsx`

Expected: PASS.

### Task 2: Runtime Resource Snapshot Model and API

**Files:**
- Modify: `internal/modules/clusteragent/domain.go`
- Modify: `internal/modules/clusteragent/migrations.go`
- Modify: `internal/modules/clusteragent/repository_mysql.go`
- Modify: `internal/modules/clusteragent/service.go`
- Modify: `internal/modules/clusteragent/api.go`
- Modify: `internal/modules/clusteragent/service_test.go`

- [x] **Step 1: Write failing API/service tests**

Add tests for storing agent-reported runtime resources and querying resources by `application_id`, `environment_id`, and `stage_key` without exposing credentials.

- [x] **Step 2: Verify RED**

Run: `go test -count=1 ./internal/modules/clusteragent -run 'TestRuntimeResource' -v`

Expected: FAIL because runtime resource types/API do not exist.

- [x] **Step 3: Implement resource domain and repository**

Create normalized runtime resource tables for latest resource status and recent events. Store resource kind/name/namespace/group/version/parent refs/status/message/container summaries.

- [x] **Step 4: Implement query API**

Expose PaaS user-facing endpoints under `/api/apps/{appId}/stages/{stageKey}/runtime/resources` and `/api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}` with permission checks.

- [x] **Step 5: Verify GREEN**

Run: `go test -count=1 ./internal/modules/clusteragent -run 'TestRuntimeResource' -v`

Expected: PASS.

### Task 3: Agent Resource Collection

**Files:**
- Modify: `internal/paasagent/domain.go`
- Modify: `internal/paasagent/kubernetes_reader.go`
- Modify: `internal/paasagent/kubernetes_reader_test.go`
- Modify: `deploy/paas-agent/templates/rbac.yaml`

- [x] **Step 1: Write failing agent tests**

Extend fake Kubernetes tests to assert Deployment, StatefulSet, DaemonSet, ReplicaSet, Pod, container state, and Event data are reported as runtime resources.

- [x] **Step 2: Verify RED**

Run: `go test -count=1 ./internal/paasagent -run 'TestKubernetesReader.*RuntimeResource' -v`

Expected: FAIL because resource detail fields are not reported.

- [x] **Step 3: Implement collection**

Extend snapshots with normalized runtime resources while preserving existing `Applications`, `Workloads`, and `Events` fields.

- [x] **Step 4: Verify GREEN**

Run: `go test -count=1 ./internal/paasagent -run 'TestKubernetesReader.*RuntimeResource' -v`

Expected: PASS.

### Task 4: Runtime Actions, Logs, and Terminal Gates

**Files:**
- Modify: `internal/modules/identityaccess/domain.go`
- Modify: `internal/modules/clusteragent/domain.go`
- Modify: `internal/modules/clusteragent/service.go`
- Modify: `internal/modules/clusteragent/api.go`
- Modify: `internal/modules/clusteragent/service_test.go`
- Modify: `internal/paasagent/agent.go`
- Modify: `internal/paasagent/kubernetes_reader.go`
- Modify: `deploy/paas-agent/templates/rbac.yaml`

- [x] **Step 1: Write failing permission and task tests**

Add tests proving restart requires `runtime:restart`, terminal requires `runtime:terminal`, terminal is denied for ordinary developers, and task creation records audit-safe target metadata.

- [x] **Step 2: Verify RED**

Run: `go test -count=1 ./internal/modules/identityaccess ./internal/modules/clusteragent -run 'Test.*Runtime' -v`

Expected: FAIL because permissions and tasks do not exist.

- [x] **Step 3: Implement permissions**

Add `runtime:read`, `runtime:restart`, and `runtime:terminal`; grant terminal only to `platform_admin` and `operator` by default.

- [x] **Step 4: Implement restart task**

Create `runtime_restart` cluster task with resource kind/name/namespace payload and agent execution via controlled patch/restart behavior.

- [x] **Step 5: Add log and terminal API surfaces**

Add API handlers and service stubs for pod logs and terminal sessions with permission checks. If full WebSocket byte streaming is too large for this slice, return a structured `501` capability response while preserving authorization and route shape.

- [x] **Step 6: Verify GREEN**

Run: `go test -count=1 ./internal/modules/identityaccess ./internal/modules/clusteragent ./internal/paasagent -run 'Test.*Runtime' -v`

Expected: PASS.

### Task 5: Web Console Resource Detail View

**Files:**
- Modify: `web/console/src/api/mock.ts`
- Modify: `web/console/src/api/index.ts`
- Modify: `web/console/src/pages/PromotionPage.tsx`
- Modify: `web/console/src/pages/PromotionPage.test.tsx`
- Modify: `web/console/src/styles.css`

- [x] **Step 1: Write failing UI tests**

Add tests proving clicking a Stage opens runtime resources, resource rows show Workload/Pod/Event status, restart is shown only for supported workloads, logs are accessible for pods, and terminal is hidden/disabled without permission.

- [x] **Step 2: Verify RED**

Run: `cd web/console && npm test -- src/pages/PromotionPage.test.tsx`

Expected: FAIL because runtime resource UI does not exist.

- [x] **Step 3: Implement API client and mock data**

Add runtime resource types and API calls with mock data matching backend JSON.

- [x] **Step 4: Implement UI**

Add a modal or drawer from Stage card to display a resource tree/list and detail panel using Chinese labels and current console visual style.

- [x] **Step 5: Verify GREEN**

Run: `cd web/console && npm test -- src/pages/PromotionPage.test.tsx`

Expected: PASS.

### Task 6: Final Verification

**Files:**
- All changed files.

- [x] **Step 1: Format**

Run: `gofmt -w` on changed Go files.

- [x] **Step 2: Focused backend tests**

Run relevant focused `go test -count=1` commands for changed backend packages.

- [x] **Step 3: Focused frontend tests**

Run: `cd web/console && npm test -- src/pages/PromotionPage.test.tsx src/api/index.workload.test.ts`

- [x] **Step 4: Full verification**

Run: `scripts/test-full.sh`

Expected: all Go packages pass, Vitest passes, build succeeds.
