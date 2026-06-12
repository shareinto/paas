# paas-agent Argo CD Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and publish an ARM64 `paas-agent` image, register the cluster in `paas-server`, and deploy `paas-agent` through the manifests Argo CD flow.

**Architecture:** Reuse the existing Helm chart under `deploy/paas-agent` as the source of truth, mirror it into the manifests chart repository, and add an environment Application under `llt-applications`. Runtime configuration is injected through Helm values, including the control-plane URL, registered cluster ID, and agent token.

**Tech Stack:** Go, Docker buildx, Helm, Kubernetes, Argo CD Application manifests, Git.

---

## File Structure

- Create or update: `/windows/go/src/github.com/shareinto/manifests/paas-agent`
  - Holds the Helm chart copied from `/windows/go/src/github.com/shareinto/paas/deploy/paas-agent`.
- Create: `/windows/go/src/github.com/shareinto/manifests/llt-applications/paas-agent-application.yaml`
  - Argo CD Application for the `llt` environment.
- Use: `/windows/go/src/github.com/shareinto/paas/deploy/paas-agent/Dockerfile`
  - Build input for the ARM64 image.
- No application code changes are required.

### Task 1: Build and Push ARM64 Image

**Files:**
- Use: `/windows/go/src/github.com/shareinto/paas/deploy/paas-agent/Dockerfile`

- [ ] **Step 1: Compute image tag**

Run:

```bash
short_sha="$(git rev-parse --short HEAD)"
image_tag="$(date +%Y%m%d)-${short_sha}-arm64"
printf '%s\n' "$image_tag"
```

Expected: prints a tag like `20260612-8298bec-arm64`.

- [ ] **Step 2: Build and push ARM64 image**

Run:

```bash
docker buildx build \
  --platform linux/arm64 \
  -f deploy/paas-agent/Dockerfile \
  -t "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/paas-agent:${image_tag}" \
  --push \
  .
```

Expected: command exits 0 and reports the pushed image digest.

### Task 2: Register Cluster in paas-server

**Files:**
- No file edits.

- [ ] **Step 1: Log in to paas-server**

Run:

```bash
curl -fsS \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"password"}' \
  http://122.152.196.135:18080/api/auth/login
```

Expected: response includes a `token.access_token` value and a `user.id` value.

- [ ] **Step 2: Read the first tenant**

Run:

```bash
curl -fsS 'http://122.152.196.135:18080/api/tenants?page=1&page_size=1'
```

Expected: response includes at least one item with an `id`.

- [ ] **Step 3: Register the cluster**

Run:

```bash
curl -fsS \
  -H 'Content-Type: application/json' \
  -d "{\"actor\":{\"Type\":\"user\",\"ID\":\"${admin_user_id}\"},\"tenant_id\":\"${tenant_id}\",\"name\":\"llt-arm-cluster\",\"region\":\"llt\"}" \
  http://122.152.196.135:18080/api/clusters
```

Expected: response includes `cluster.id` and `agent_token`. Do not print the token in user-facing output.

### Task 3: Create manifests paas-agent Chart Repository

**Files:**
- Create or update: `/windows/go/src/github.com/shareinto/manifests/paas-agent/Chart.yaml`
- Create or update: `/windows/go/src/github.com/shareinto/manifests/paas-agent/values.yaml`
- Create or update: `/windows/go/src/github.com/shareinto/manifests/paas-agent/templates/*`

- [ ] **Step 1: Copy chart from paas repository**

Run:

```bash
mkdir -p /windows/go/src/github.com/shareinto/manifests/paas-agent
rsync -a --delete \
  /windows/go/src/github.com/shareinto/paas/deploy/paas-agent/ \
  /windows/go/src/github.com/shareinto/manifests/paas-agent/
```

Expected: manifests chart directory contains `Chart.yaml`, `values.yaml`, and `templates/`.

- [ ] **Step 2: Render values for this deployment**

Update `/windows/go/src/github.com/shareinto/manifests/paas-agent/values.yaml` so these fields match the runtime values:

```yaml
image:
  repository: cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/paas-agent
  tag: ${image_tag}
  pullPolicy: IfNotPresent

config:
  clusterID: ${cluster_id}
  controlPlaneURL: http://122.152.196.135:18080
  agentNamespaces: argocd,macc,paas-system
  argocdNamespace: argocd
  heartbeatInterval: 10s
  snapshotInterval: 30s

secret:
  existingSecret: ""
  token: ${agent_token}
```

Expected: token is present in values because the user approved writing it to manifests.

### Task 4: Add llt Argo CD Application

**Files:**
- Create: `/windows/go/src/github.com/shareinto/manifests/llt-applications/paas-agent-application.yaml`

- [ ] **Step 1: Create Application manifest**

Create:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: paas-agent
  namespace: argocd
  labels:
    app.kubernetes.io/name: paas-agent
    app.kubernetes.io/part-of: llt-applications
spec:
  project: default
  source:
    repoURL: ssh://git@gitops:2422/k8s/paas-agent.git
    targetRevision: main
    path: .
    helm:
      releaseName: paas-agent
  destination:
    server: https://kubernetes.default.svc
    namespace: paas-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
```

Expected: Application follows the existing `llt-applications` pattern.

### Task 5: Validate and Publish manifests

**Files:**
- Use: `/windows/go/src/github.com/shareinto/manifests/paas-agent`
- Use: `/windows/go/src/github.com/shareinto/manifests/llt-applications/paas-agent-application.yaml`

- [ ] **Step 1: Validate Helm chart**

Run:

```bash
helm lint /windows/go/src/github.com/shareinto/manifests/paas-agent
helm template paas-agent /windows/go/src/github.com/shareinto/manifests/paas-agent --namespace paas-system >/tmp/paas-agent-rendered.yaml
```

Expected: `helm lint` exits 0 and `helm template` exits 0.

- [ ] **Step 2: Commit and push paas-agent chart repository**

Run from `/windows/go/src/github.com/shareinto/manifests/paas-agent`:

```bash
git status --short
git add Chart.yaml values.yaml templates
git commit -m "feat: add paas-agent chart"
git push origin main
```

Expected: push exits 0.

- [ ] **Step 3: Commit and push llt Application repository**

Run from `/windows/go/src/github.com/shareinto/manifests/llt-applications`:

```bash
git status --short
git add paas-agent-application.yaml
git commit -m "feat: deploy paas-agent"
git push origin main
```

Expected: push exits 0.

### Task 6: Sync and Verify Deployment

**Files:**
- No file edits.

- [ ] **Step 1: Let Argo CD create or refresh the Application**

Run:

```bash
kubectl -n argocd get application paas-agent
```

Expected: Application exists. If it does not exist, apply the Application manifest once with `kubectl apply -f /windows/go/src/github.com/shareinto/manifests/llt-applications/paas-agent-application.yaml`.

- [ ] **Step 2: Verify rollout**

Run:

```bash
kubectl -n paas-system rollout status deployment/paas-agent --timeout=180s
kubectl -n paas-system get pods -l app.kubernetes.io/name=paas-agent
```

Expected: rollout exits 0 and at least one pod is Running or Ready.

- [ ] **Step 3: Verify agent logs**

Run:

```bash
kubectl -n paas-system logs deployment/paas-agent --tail=80
```

Expected: logs include the configured cluster ID and heartbeat/snapshot configuration, and do not print the agent token.

- [ ] **Step 4: Verify control-plane heartbeat**

Run:

```bash
curl -fsS "http://122.152.196.135:18080/api/clusters?tenant_id=${tenant_id}&page=1&page_size=20"
```

Expected: registered cluster is present and has a recent `last_heartbeat_at` after the agent starts.
