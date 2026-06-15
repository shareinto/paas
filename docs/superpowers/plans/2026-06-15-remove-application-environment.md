# Remove Application Environment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the application-level Environment model and make Application Stage a real-time projection from the tenant DeliveryFlow template.

**Architecture:** `DeliveryFlowTemplateStage` and `StageClusterBinding` are the source of truth. Application-specific data is stored by `application_id + stage_key`, including stage state, workload stage config, deployments, promotions, and runtime resources. No user-facing API or domain flow should require `environment_id`.

**Tech Stack:** Go, MySQL migrations/repositories, React, TypeScript, Vitest.

---

### Task 1: Stage State and Config Model

**Files:**
- Modify: `internal/modules/appenv/domain.go`
- Modify: `internal/modules/appenv/migrations.go`
- Modify: `internal/modules/appenv/repository_mysql.go`
- Modify: `internal/modules/appenv/service.go`
- Modify: `internal/modules/appenv/api.go`
- Test: `internal/modules/appenv/service_test.go`

- [ ] Add `ApplicationStageState`, `WorkloadStageConfig`, and stage-key based APIs.
- [ ] Remove application `Environment`, `EnvironmentClusterBinding`, `EnvironmentState`, and environment config/secret/route tables from current schema.
- [ ] Replace workload environment config persistence with `application_id + workload_id + stage_key`.
- [ ] Verify with focused appenv tests.

### Task 2: Delivery Stage Projection

**Files:**
- Modify: `internal/modules/delivery/domain.go`
- Modify: `internal/modules/delivery/ports.go`
- Modify: `internal/modules/delivery/service.go`
- Modify: `internal/modules/delivery/repository_mysql.go`
- Modify: `internal/modules/delivery/migrations.go`
- Test: `internal/modules/delivery/service_test.go`

- [ ] Make `ListAppStages` project directly from `DeliveryFlowTemplateStage`.
- [ ] Remove per-app `DeliveryStage` and `target_environment_id` dependencies from promotion flow.
- [ ] Make promotion validation use `StageClusterBinding` only.
- [ ] Verify template edits are immediately reflected in `ListAppStages`.

### Task 3: GitOps and Agent Runtime Association

**Files:**
- Modify: `internal/modules/gitops/domain.go`
- Modify: `internal/modules/gitops/ports.go`
- Modify: `internal/modules/gitops/service.go`
- Modify: `internal/modules/gitops/migrations.go`
- Modify: `internal/modules/clusteragent/*`
- Modify: `internal/paasagent/*`
- Test: `internal/modules/gitops/service_test.go`
- Test: `internal/modules/clusteragent/service_test.go`
- Test: `internal/paasagent/kubernetes_reader_test.go`

- [ ] Change deployment records and manifest revisions to store `stage_key`.
- [ ] Render Argo Application and workload labels with `application-id`, `stage-key`, and `deployment-id`; stop using `environment-id`.
- [ ] Store/query runtime resources by `application_id + stage_key`.
- [ ] Update agent status forwarding to stage state.

### Task 4: Server Wiring and Web Console

**Files:**
- Modify: `cmd/paas-server/server.go`
- Modify: `cmd/paas-server/main_test.go`
- Modify: `web/console/src/api/index.ts`
- Modify: `web/console/src/api/mock.ts`
- Modify: `web/console/src/pages/ApplicationDetailPage.tsx`
- Modify: `web/console/src/pages/PromotionPage.tsx`
- Test: related frontend tests

- [ ] Remove environment bridges and wire stage-state/stage-config bridges.
- [ ] Remove `environment_id` from frontend API calls and DTOs.
- [ ] Keep all user-facing text in Chinese and use Stage terminology only.

### Task 5: Docs and Verification

**Files:**
- Modify: `doc/需求.md`
- Modify: `doc/总体设计.md`
- Modify: `doc/模块划分.md`

- [ ] Update docs to remove application Environment.
- [ ] Run focused Go package tests.
- [ ] Run affected Vitest suites.
- [ ] Run `scripts/test-full.sh`.
