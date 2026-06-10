# Java 构建适配方案

## 1. 背景

客户源码仓库的目录结构、构建工具和产物组织方式差异较大。当前测试可用版本不要求 Jenkins 自动猜测所有情况，也不允许客户直接暴露 Jenkinsfile 给平台执行。

平台采用“PaaS 固化构建规格，Jenkins 执行统一模板”的方式：

- PaaS 在创建或编辑 Application 时生成并保存 `BuildSpec`。
- Jenkins 只运行平台托管的统一 Job 模板。
- Jenkins 根据 `BuildSpec` 执行受限构建命令、收集构建产物，并使用平台标准运行时镜像打包容器镜像。

## 2. 测试可用版本支持范围

当前测试可用版本支持 Java 应用构建，管理员通过构建环境维护构建镜像。默认构建环境如下：

```text
java-springboot
java-tomcat
```

含义：

- `java-springboot`：使用 Maven Java 构建镜像。
- `java-tomcat`：使用 Tomcat 兼容 Java 构建镜像。

当前测试可用版本暂不支持：

- Go、Python 等其他语言。
- 客户仓库自带 Jenkinsfile。
- 客户源码仓库内的 Dockerfile 作为镜像构建入口。
- Jenkins UI 对普通用户开放。

## 3. BuildSpec

`BuildSpec` 是 PaaS 固化后的单代码源构建规格，应保存在对应 `ApplicationSource` 中。一个 Application 可以包含多个 ApplicationSource，一次 Build 会按各代码源的 BuildSpec 检出、构建并收集产物。

字段：

```text
source_path
build_command
artifact_copy_command
runtime_base_image
artifact_deploy_path
default_ref
```

字段说明：

- `source_path`：应用源码在 SourceRepository 中的相对路径。
- `build_command`：用户填写的受限构建命令。
- `artifact_copy_command`：必填产物拷贝命令，用于把当前代码源构建产物复制到平台约定的 `$PAAS_ARTIFACT_OUTPUT` 目录。
- `runtime_base_image`：平台维护的运行时基础镜像；创建构建流水线时按流水线配置一次。
- `artifact_deploy_path`：可选产物放置路径，表示构建产物在运行时镜像或容器内的目标位置，创建构建流水线时按流水线配置一次。
- `default_ref`：默认构建分支、tag 或 commit 引用。

产物规则：

- 每个 ApplicationSource 至少声明一个产物拷贝命令。
- 一次 Build 可以产生多个 BuildArtifact，产物通过 `source_key` 关联到代码源。
- 构建成功回调使用 `artifacts` 数组上报多个产物；`is_primary=true` 的产物作为主产物展示。

- `artifact_copy_command` 必须把最终要进入镜像上下文的文件写入 `$PAAS_ARTIFACT_OUTPUT`。
- 收集后平台会校验 `$PAAS_ARTIFACT_OUTPUT` 非空。

## 4. 创建构建流水线

创建 Application 时只维护应用基础信息和默认环境。创建 BuildPipeline 时，代码源配置块负责让用户为每个代码源选择“构建环境”，流水线级运行时配置负责让用户选择一个或多个“运行时环境”。

用户需要填写：

```text
流水线标识：main / release
运行时环境：Java 17 运行时 / Tomcat 8 运行时
构建环境：java-springboot / java-tomcat
源码子目录：source_path
构建命令：build_command
产物拷贝命令：artifact_copy_command
```

示例：

```text
Spring Boot:
  source_path = services/user-api
  build_command = mvn clean package -DskipTests
  artifact_copy_command = cp -ar target/user-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"

Tomcat:
  source_path = apps/legacy-web
  build_command = mvn clean package -DskipTests
  artifact_copy_command = cp -ar target/legacy-web.war "$PAAS_ARTIFACT_OUTPUT/app.war"
```

PaaS 校验：

- `source_path` 必须存在或可被 GitLab 仓库 API 验证。
- 每个代码源必须选择已启用的构建环境，构建环境提供构建镜像。
- 每条构建流水线必须选择一个或多个已启用的运行时环境；首个运行时环境作为主产物镜像目标。
- `build_command` 不能为空。
- `runtime_base_image` 来自构建流水线选择的运行时环境；不在创建应用页面填写。
- `artifact_deploy_path` 填写时必须是绝对路径，且不允许通过 `..` 路径逃逸。

## 5. 仓库扫描建议

SourceRepository 迁移或扫描后，PaaS 只生成建议，不自动决定最终配置。

扫描内容：

```text
pom.xml
build.gradle
target/*.jar
target/*.war
```

建议内容：

```text
source_path
build_command
artifact_copy_command
runtime_base_image
```

用户在创建 BuildPipeline 时必须确认或调整建议，确认后才写入 `BuildPipelineSource` / `BuildSpec`。

## 6. 构建管理和全局模板

平台管理提供“构建管理”，包含三个子页面：

```text
构建环境
运行时环境
构建模板
```

- 构建环境：维护可选构建预设，例如 `java-springboot`、`java-tomcat`，包含名称、描述和构建镜像。
- 运行时环境：维护名称、运行时基础镜像、产物放置路径和 Dockerfile 路径。Dockerfile 路径仅在运行时环境管理页由平台管理员维护，创建应用、创建流水线和触发构建时不向用户展示。
- 构建模板：全局仅维护一个模板。平台根据用户填写的代码源、构建环境、运行时环境和平台 Dockerfile 仓库配置渲染最终 Jenkins Job。

Jenkins 仍由 PaaS 创建和管理。Application 创建时不创建 Job；用户点击构建时使用固定名称创建或更新 Job。推荐 Job 名称：

```text
paas/{tenant_name}/{project_name}/{app_name}
```

PaaS 每次触发构建前根据最终流水线配置快照渲染 Jenkins Job。Job 不存在时创建，已存在时更新配置，然后触发构建；不再计算 md5 指纹，也不再按配置变化替换 Job。

Jenkins Job 渲染：

```text
当前测试可用版本不向 Jenkins 传递构建参数。PaaS 每次触发构建前都会按本次 BuildRun 重新渲染 Jenkinsfile，并更新固定 Jenkins Job。
```

运行时环境、源码 ref、commit、镜像仓库、应用名、Dockerfile 路径、产物放置路径和回调地址都由 PaaS 渲染进 Jenkinsfile。模板本身只遍历 PaaS 渲染出的镜像目标，不写死 ACK/AWS 名称。
平台 Dockerfile 仓库应提供 `java/jar/Dockerfile` 和 `java/tomcat/Dockerfile`，参考样例见 `doc/dockerfiles/java/jar/Dockerfile` 和 `doc/dockerfiles/java/tomcat/Dockerfile`。

Spring Boot jar 场景约定 `artifact_copy_command` 把主 jar 写为 `$PAAS_ARTIFACT_OUTPUT/app.jar`。Tomcat war 场景约定写为 `$PAAS_ARTIFACT_OUTPUT/app.war`，并使用运行时环境的 `artifact_deploy_path` 作为放置目录。镜像上下文准备阶段会把完整 `artifact/` 目录复制到每个 `image-context/{runtime_key}/`。

模板流程：

```text
1. 清理本次构建输出目录，保留 `source/` 和 `.paas/dockerfiles`，便于复用 Git 工作树。
2. 增量检出平台 Dockerfile 仓库：已有 `.git` 时执行 `git fetch --prune --tags origin`，再按 ref 解析 commit、`checkout --detach`、`reset --hard`、`clean -fdx`。
3. 对每个 ApplicationSource 渲染独立的“检出 {source_key}”阶段。
4. 每个 ApplicationSource 的源码目录也采用增量检出，首次 `git clone --no-checkout`，后续复用工作树并 fetch 最新 ref。
5. 对每个 ApplicationSource 渲染独立的“构建 {source_key}”阶段，在 `source_path` 内执行用户 BuildSpec 的 `build_command`。
6. 构建容器挂载 Jenkins 节点持久化缓存目录 `/backup_data/paas-cache/dependencies/{cache_key}/{source_key}`，并设置 Maven、Gradle、npm、Yarn、pnpm 的常用缓存路径。
7. 对每个 ApplicationSource 渲染独立的“收集产物 {source_key}”阶段，把产物放入平台约定输出目录。
8. 使用 PaaS 渲染出的运行时和镜像目标准备镜像上下文。
9. 使用 `docker buildx build --platform` 按镜像目标生成并推送多架构镜像，第一个目标为当前版本主产物。
10. buildx 本地缓存读取 `/backup_data/buildx-cache/{job_name}/{target_key}`，写入临时目录 `{cache_dir}.next`，构建成功后再替换原缓存目录，避免失败构建破坏可用缓存。
11. 通过 `curl` 回调 PaaS 控制面。
```

模板渲染示例：

```text
ApplicationSource[frontcomponents]
  -> stage('检出 frontcomponents')
  -> stage('构建 frontcomponents')
  -> stage('收集产物 frontcomponents')

ApplicationSource[frontmacc5]
  -> stage('检出 frontmacc5')
  -> stage('构建 frontmacc5')
  -> stage('收集产物 frontmacc5')

全局收尾
  -> stage('准备镜像上下文')
  -> stage('初始化 buildx')
  -> stage('生成并推送多架构镜像')
```

编译由用户 BuildSpec 的受限命令执行；镜像生成由平台统一 Jenkins 模板执行。当前实现不依赖 `paas-ci-helper`，模板直接使用 `docker buildx` 构建多架构镜像，并使用 `curl` 回调 PaaS 控制面。源码、Dockerfile 仓库、依赖缓存和 buildx layer cache 都是 Jenkins 节点本地缓存；如果同一个 Job 在不同节点间调度，缓存命中率取决于节点亲和性和 `/backup_data` 的持久化方式。

## 7. 安全策略

构建命令安全策略：

- `build_command` 只在 `source_path` 目录内执行。
- Jenkins agent 必须使用隔离容器或隔离节点。
- 构建凭据只通过 Jenkins 受控凭据注入。
- 构建命令、产物拷贝命令、运行时基础镜像的变更必须记录审计日志。
- 构建日志必须脱敏平台 token、GitLab token、镜像仓库密码等敏感信息。
- 产物拷贝命令与构建命令一样，只能在 `source_path` 目录内执行，并由 Jenkins 模板提供受控的产物输出目录。
- 不向前端返回 Jenkins 凭据、GitLab 凭据、镜像仓库凭据。

## 8. 测试要求

Spring Boot 场景：

- Maven 项目执行用户命令后生成 jar。
- Jenkins 使用平台 Spring Boot 基础镜像打包并推送镜像。
- 构建成功后生成 BuildArtifact、Release、Freight。

Tomcat 场景：

- Maven 项目执行用户命令后生成 war。
- Jenkins 使用平台 Tomcat 基础镜像打包并推送镜像。

Monorepo 场景：

- 同一 SourceRepository 下不同 `source_path` 创建多个 Application。
- 每个 Application 使用独立构建命令和产物拷贝命令。

失败场景：

- `artifact_copy_command` 没有向 `$PAAS_ARTIFACT_OUTPUT` 写入任何文件。
- 构建命令退出码非 0。
- Jenkins Job 创建失败。
- 平台 Dockerfile 仓库检出失败或默认 Dockerfile 不存在。
- 镜像推送失败。
- 构建日志脱敏失败。

安全场景：

- 构建命令变更产生审计日志。
- Jenkins 凭据不返回给前端。
- 用户无法访问 Jenkins UI。
