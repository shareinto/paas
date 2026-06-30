package build

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo            Repository
	apps            ApplicationQuery
	gitSources      GitSourceQuery
	workloads       WorkloadQuery
	runner          BuildRunnerPort
	permission      PermissionChecker
	audit           AuditLogger
	events          EventPublisher
	runtimeSyncer   RuntimeEnvironmentSyncer
	ids             shared.IDGenerator
	clock           shared.Clock
	templateID      string
	callbackURL     string
	imageRepository string
	dockerfileRepo  DockerfileRepositoryConfig
	sensitiveValues []string
}

const maxProgressiveLogDrainIterations = 50
const currentDefaultBuildTemplateVersion = 9
const defaultSVNCredentialsID = "aea4ebeb-2858-4850-b3e0-268e9aff9726"
const defaultGitCredentialsID = "aea4ebeb-2858-4850-b3e0-268e9aff9726"

type buildLogDrainResult struct {
	run      BuildRun
	events   []LogEvent
	complete bool
}

type Options struct {
	Repository           Repository
	ApplicationQuery     ApplicationQuery
	GitSourceQuery       GitSourceQuery
	WorkloadQuery        WorkloadQuery
	BuildRunner          BuildRunnerPort
	PermissionChecker    PermissionChecker
	Audit                AuditLogger
	EventPublisher       EventPublisher
	RuntimeSyncer        RuntimeEnvironmentSyncer
	IDGenerator          shared.IDGenerator
	Clock                shared.Clock
	TemplateID           string
	CallbackURL          string
	ImageRepository      string
	DockerfileRepository DockerfileRepositoryConfig
	SensitiveValues      []string
}

type DockerfileRepositoryConfig struct {
	URL           string `json:"url"`
	Ref           string `json:"ref"`
	CredentialsID string `json:"credentials_id"`
}

func NewService(opts Options) *Service {
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	events := opts.EventPublisher
	if events == nil {
		events = NoopEventPublisher{}
	}
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	templateID := strings.TrimSpace(opts.TemplateID)
	if templateID == "" {
		templateID = "java-unified-v1"
	}
	return &Service{
		repo:            opts.Repository,
		apps:            opts.ApplicationQuery,
		gitSources:      opts.GitSourceQuery,
		workloads:       opts.WorkloadQuery,
		runner:          opts.BuildRunner,
		permission:      opts.PermissionChecker,
		audit:           audit,
		events:          events,
		runtimeSyncer:   opts.RuntimeSyncer,
		ids:             ids,
		clock:           clock,
		templateID:      templateID,
		callbackURL:     strings.TrimSpace(opts.CallbackURL),
		imageRepository: strings.TrimRight(strings.TrimSpace(opts.ImageRepository), "/"),
		dockerfileRepo:  normalizeDockerfileRepositoryConfig(opts.DockerfileRepository),
		sensitiveValues: normalizeSensitiveValues(opts.SensitiveValues),
	}
}

func (s *Service) SetApplicationQuery(query ApplicationQuery) {
	s.apps = query
}

func (s *Service) SetWorkloadQuery(query WorkloadQuery) {
	s.workloads = query
}

func (s *Service) SetRuntimeEnvironmentSyncer(syncer RuntimeEnvironmentSyncer) {
	s.runtimeSyncer = syncer
}

type TriggerBuildInput struct {
	Actor         identityaccess.Subject    `json:"actor"`
	PipelineID    shared.ID                 `json:"pipeline_id"`
	ApplicationID shared.ID                 `json:"application_id"`
	Sources       []TriggerBuildSourceInput `json:"sources"`
	SourceRef     string                    `json:"source_ref,omitempty"`
	CommitSHA     string                    `json:"commit_sha,omitempty"`
	Version       string                    `json:"version,omitempty"`
}

type TriggerBuildSourceInput struct {
	Key       string `json:"key"`
	SourceRef string `json:"source_ref"`
	CommitSHA string `json:"commit_sha"`
}

type CreateBuildPipelineInput struct {
	Actor                 identityaccess.Subject     `json:"actor"`
	ApplicationID         shared.ID                  `json:"application_id"`
	Name                  string                     `json:"name"`
	DisplayName           string                     `json:"display_name"`
	Description           string                     `json:"description"`
	RuntimeEnvironmentIDs []shared.ID                `json:"runtime_environment_ids"`
	Sources               []BuildPipelineSourceInput `json:"sources"`
}

type UpdateBuildPipelineInput struct {
	Actor                 identityaccess.Subject     `json:"actor"`
	PipelineID            shared.ID                  `json:"pipeline_id"`
	DisplayName           string                     `json:"display_name"`
	Description           string                     `json:"description"`
	RuntimeEnvironmentIDs []shared.ID                `json:"runtime_environment_ids"`
	Sources               []BuildPipelineSourceInput `json:"sources"`
}

type BuildPipelineSourceInput struct {
	Key                string            `json:"key"`
	DisplayName        string            `json:"display_name"`
	SourceType         SourceType        `json:"source_type"`
	SourceURL          string            `json:"source_url"`
	SourceRef          string            `json:"source_ref"`
	SVNRevision        string            `json:"svn_revision,omitempty"`
	SVNCheckoutPaths   []SVNCheckoutPath `json:"svn_checkout_paths,omitempty"`
	BuildEnvironmentID shared.ID         `json:"build_environment_id"`
	SourcePath         string            `json:"source_path"`
	BuildSpec          BuildSpec         `json:"build_spec"`
	DefaultRef         string            `json:"default_ref"`
	IsPrimary          bool              `json:"is_primary"`
}

type ListGitBranchesInput struct {
	Actor     identityaccess.Subject `json:"actor"`
	ProjectID shared.ID              `json:"project_id"`
	SourceURL string                 `json:"source_url"`
}

type CreateJenkinsJobTemplateInput struct {
	Actor              identityaccess.Subject `json:"actor"`
	Name               string                 `json:"name"`
	JenkinsfileContent string                 `json:"jenkinsfile_content"`
	XMLContent         string                 `json:"xml_content"`
	IsDefault          bool                   `json:"is_default"`
}

type UpdateJenkinsJobTemplateInput struct {
	Actor      identityaccess.Subject   `json:"actor"`
	TemplateID shared.ID                `json:"template_id"`
	Status     JenkinsJobTemplateStatus `json:"status"`
	IsDefault  bool                     `json:"is_default"`
}

type UploadJenkinsJobTemplateRevisionInput struct {
	Actor              identityaccess.Subject `json:"actor"`
	TemplateID         shared.ID              `json:"template_id"`
	JenkinsfileContent string                 `json:"jenkinsfile_content"`
	XMLContent         string                 `json:"xml_content"`
}

type CreateBuildEnvironmentInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	BuildImage  string                 `json:"build_image"`
	IsDefault   bool                   `json:"is_default"`
}

type UpdateBuildEnvironmentInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	EnvironmentID shared.ID              `json:"environment_id"`
	Description   string                 `json:"description"`
	BuildImage    string                 `json:"build_image"`
	Status        BuildEnvironmentStatus `json:"status"`
	IsDefault     bool                   `json:"is_default"`
}

type CreateRuntimeEnvironmentInput struct {
	Actor              identityaccess.Subject    `json:"actor"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	RuntimeBaseImage   string                    `json:"runtime_base_image"`
	ArtifactDeployPath string                    `json:"artifact_deploy_path"`
	DockerfilePath     string                    `json:"dockerfile_path"`
	SelectorLabels     map[string]string         `json:"selector_labels"`
	Images             []RuntimeEnvironmentImage `json:"images"`
}

type UpdateRuntimeEnvironmentInput struct {
	Actor              identityaccess.Subject    `json:"actor"`
	EnvironmentID      shared.ID                 `json:"environment_id"`
	Description        string                    `json:"description"`
	RuntimeBaseImage   string                    `json:"runtime_base_image"`
	ArtifactDeployPath string                    `json:"artifact_deploy_path"`
	DockerfilePath     string                    `json:"dockerfile_path"`
	SelectorLabels     map[string]string         `json:"selector_labels"`
	Images             []RuntimeEnvironmentImage `json:"images"`
	Status             RuntimeEnvironmentStatus  `json:"status"`
}

type SaveBuildTemplateInput struct {
	Actor   identityaccess.Subject `json:"actor"`
	Content string                 `json:"content"`
}

type BuildCallbackInput struct {
	BuildRunID         shared.ID                    `json:"build_run_id"`
	Status             BuildRunStatus               `json:"status"`
	JenkinsBuildNumber int64                        `json:"jenkins_build_number"`
	CommitSHA          string                       `json:"commit_sha"`
	Artifacts          []BuildCallbackArtifactInput `json:"artifacts"`
	ImageURI           string                       `json:"image_uri,omitempty"`
	ImageDigest        string                       `json:"image_digest,omitempty"`
	ErrorMessage       string                       `json:"error_message"`
}

type BuildCallbackArtifactInput struct {
	SourceKey      string            `json:"source_key"`
	ContainerName  string            `json:"container_name"`
	Type           BuildArtifactType `json:"type"`
	Name           string            `json:"name"`
	URI            string            `json:"uri"`
	Digest         string            `json:"digest"`
	IsPrimary      bool              `json:"is_primary"`
	SelectorLabels map[string]string `json:"selector_labels"`
	Metadata       map[string]string `json:"metadata"`
}

type LogEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

const defaultJenkinsfile = `pipeline {
  agent any
  stages {
    stage('构建') {
      steps {
        sh 'echo "请在 PaaS 构建模板中配置具体流水线"'
      }
    }
  }
}`

const defaultBuildTemplateContent = `pipeline {
  agent any

  options {
    buildDiscarder(logRotator(numToKeepStr: '10'))
    disableConcurrentBuilds()
    timestamps()
  }

  environment {
    PAAS_CALLBACK_URL = '{{ .CallbackURL }}'
  }

  stages {
    stage('cleanup-workspace') {
      steps {
        sh '''
          set -eu
          rm -rf artifact image-context report
          mkdir -p source artifact image-context report .paas/dockerfiles
        '''
      }
    }
{{ if .DockerfileRepository.URL }}
    stage('checkout-dockerfiles') {
      steps {
{{ if .DockerfileRepository.CredentialsID }}
        withCredentials([usernamePassword(credentialsId: '{{ .DockerfileRepository.CredentialsID }}', usernameVariable: 'PAAS_DOCKERFILE_GIT_USER', passwordVariable: 'PAAS_DOCKERFILE_GIT_PASSWORD')]) {
          sh '''
            set -eu
            askpass="$WORKSPACE/.paas/dockerfile-git-askpass"
            cat > "$askpass" <<'ASKPASS'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' "$PAAS_DOCKERFILE_GIT_USER" ;;
  *) printf '%s\n' "$PAAS_DOCKERFILE_GIT_PASSWORD" ;;
esac
ASKPASS
            chmod 700 "$askpass"
            export GIT_ASKPASS="$askpass"
            export GIT_TERMINAL_PROMPT=0
            repo_url='{{ .DockerfileRepository.URL }}'
            checkout_dir='.paas/dockerfiles'
            ref='{{ .DockerfileRepository.Ref }}'
            mkdir -p "$(dirname "$checkout_dir")"
            if [ -d "$checkout_dir/.git" ]; then
              git -C "$checkout_dir" remote set-url origin "$repo_url"
              git -C "$checkout_dir" fetch --prune --tags origin
            else
              rm -rf "$checkout_dir"
              git clone --no-checkout "$repo_url" "$checkout_dir"
              git -C "$checkout_dir" fetch --prune --tags origin
            fi
            if git -C "$checkout_dir" rev-parse --verify --quiet "origin/$ref^{commit}" >/dev/null; then
              commit="$(git -C "$checkout_dir" rev-parse "origin/$ref^{commit}")"
            else
              commit="$(git -C "$checkout_dir" rev-parse "$ref^{commit}")"
            fi
            git -C "$checkout_dir" checkout --detach "$commit"
            git -C "$checkout_dir" reset --hard "$commit"
            git -C "$checkout_dir" clean -fdx
          '''
        }
{{ else }}
        sh '''
          set -eu
          repo_url='{{ .DockerfileRepository.URL }}'
          checkout_dir='.paas/dockerfiles'
          ref='{{ .DockerfileRepository.Ref }}'
          mkdir -p "$(dirname "$checkout_dir")"
          if [ -d "$checkout_dir/.git" ]; then
            git -C "$checkout_dir" remote set-url origin "$repo_url"
            git -C "$checkout_dir" fetch --prune --tags origin
          else
            rm -rf "$checkout_dir"
            git clone --no-checkout "$repo_url" "$checkout_dir"
            git -C "$checkout_dir" fetch --prune --tags origin
          fi
          if git -C "$checkout_dir" rev-parse --verify --quiet "origin/$ref^{commit}" >/dev/null; then
            commit="$(git -C "$checkout_dir" rev-parse "origin/$ref^{commit}")"
          else
            commit="$(git -C "$checkout_dir" rev-parse "$ref^{commit}")"
          fi
          git -C "$checkout_dir" checkout --detach "$commit"
          git -C "$checkout_dir" reset --hard "$commit"
          git -C "$checkout_dir" clean -fdx
        '''
{{ end }}
      }
    }
{{ else }}
    stage('checkout-dockerfiles') {
      steps {
        sh 'echo "platform Dockerfile repository is not configured" >&2; exit 1'
      }
    }
{{ end }}
{{ range .Sources }}
    stage('checkout {{ .Key }}') {
      steps {
        dir('{{ .CheckoutDir }}') {
{{ if eq .SourceType "svn" }}
          withCredentials([usernamePassword(credentialsId: '{{ .SVNCredentials }}', usernameVariable: 'PAAS_SVN_USER', passwordVariable: 'PAAS_SVN_PASSWORD')]) {
            sh '''
              set -eu
              repo_url='{{ .RepoURL }}'
              svn_revision='{{ .SVNRevision }}'
              if [ -z "$svn_revision" ]; then
                svn_revision='HEAD'
              fi
              find . -mindepth 1 -maxdepth 1 -exec rm -rf {} +
              actual_revision=''
              svn_checkout_one() {
                local_dir="$1"
                relative_path="$2"
                depth="$3"
                remote_url="${repo_url%/}"
                if [ -n "$relative_path" ]; then
                  remote_url="${remote_url}/${relative_path}"
                fi
                remote_url="${remote_url}@${svn_revision}"
                if [ "$local_dir" != "." ]; then
                  rm -rf "$local_dir"
                  mkdir -p "$(dirname "$local_dir")"
                fi
                svn checkout --username "$PAAS_SVN_USER" --password "$PAAS_SVN_PASSWORD" --non-interactive --trust-server-cert --depth "$depth" "$remote_url" "$local_dir"
                if [ -z "$actual_revision" ]; then
                  actual_revision="$(svn info --show-item revision "$local_dir" 2>/dev/null || true)"
                fi
              }
{{ range .SVNCheckoutPaths }}
              svn_checkout_one '{{ .Local }}' '{{ .Path }}' '{{ .Depth }}'
{{ end }}
              mkdir -p "$WORKSPACE/report"
              printf '%s\n' "$actual_revision" > "$WORKSPACE/report/source-{{ .Key }}-commit.txt"
              printf '%s\n' "$actual_revision" > "$WORKSPACE/report/source-{{ .Key }}-svn-revision.txt"
            '''
          }
{{ else }}
          withCredentials([usernamePassword(credentialsId: '{{ .GitCredentials }}', usernameVariable: 'PAAS_GIT_USER', passwordVariable: 'PAAS_GIT_PASSWORD')]) {
            sh '''
              set -eu
              askpass="$WORKSPACE/.paas/source-{{ .Key }}-git-askpass"
              cat > "$askpass" <<'ASKPASS'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' "$PAAS_GIT_USER" ;;
  *) printf '%s\n' "$PAAS_GIT_PASSWORD" ;;
esac
ASKPASS
              chmod 700 "$askpass"
              export GIT_ASKPASS="$askpass"
              export GIT_TERMINAL_PROMPT=0
              repo_url='{{ .RepoURL }}'
              ref_b64='{{ .SourceRefB64 }}'
              if ref="$(printf '%s' "$ref_b64" | base64 -d 2>/dev/null)"; then
                :
              else
                ref="$(printf '%s' "$ref_b64" | base64 --decode)"
              fi
              if [ -d .git ]; then
                git remote set-url origin "$repo_url"
                git fetch --prune --tags origin
              else
                find . -mindepth 1 -maxdepth 1 -exec rm -rf {} +
                git clone --no-checkout "$repo_url" .
                git fetch --prune --tags origin
              fi
              if git rev-parse --verify --quiet "origin/$ref^{commit}" >/dev/null; then
                commit="$(git rev-parse "origin/$ref^{commit}")"
              else
                commit="$(git rev-parse "$ref^{commit}")"
              fi
              git checkout --detach "$commit"
              git reset --hard "$commit"
              git clean -fdx
              mkdir -p "$WORKSPACE/report"
              printf '%s\n' "$commit" > "$WORKSPACE/report/source-{{ .Key }}-commit.txt"
            '''
          }
{{ end }}
        }
      }
    }

    stage('build {{ .Key }}') {
      steps {
        script {
          def PAAS_DEP_CACHE = sh(script: '''
            set -eu
            cache_key=$(printf '%s/%s' "${JOB_NAME:-paas}" '{{ .Key }}' | sha256sum | awk '{print $1}' | cut -c1-16)
            printf '/backup_data/paas-cache/dependencies/%s/{{ .Key }}' "$cache_key"
          ''', returnStdout: true).trim()
          sh "mkdir -p \"$PAAS_DEP_CACHE\""
          withEnv(["PAAS_CACHE_ROOT=/backup_data/paas-cache", "PAAS_DEP_CACHE=${PAAS_DEP_CACHE}"]) {
            docker.image('{{ .BuildImage }}').inside("-v ${PAAS_DEP_CACHE}:${PAAS_DEP_CACHE}") {
              dir('{{ .WorkDir }}') {
                sh '''
                  set -eu
                  mkdir -p "$PAAS_DEP_CACHE/maven/repository" "$PAAS_DEP_CACHE/gradle" "$PAAS_DEP_CACHE/npm" "$PAAS_DEP_CACHE/yarn" "$PAAS_DEP_CACHE/pnpm-store"
                  export MAVEN_OPTS="-Dmaven.repo.local=$PAAS_DEP_CACHE/maven/repository ${MAVEN_OPTS:-}"
                  export GRADLE_USER_HOME="$PAAS_DEP_CACHE/gradle"
                  export NPM_CONFIG_CACHE="$PAAS_DEP_CACHE/npm"
                  export YARN_CACHE_FOLDER="$PAAS_DEP_CACHE/yarn"
                  if command -v pnpm >/dev/null 2>&1; then
                    pnpm config set store-dir "$PAAS_DEP_CACHE/pnpm-store"
                  fi
{{ .BuildCommand }}
                '''
              }
            }
          }
        }
      }
    }

    stage('collect {{ .Key }}') {
      steps {
        script {
          def PAAS_DEP_CACHE = sh(script: '''
            set -eu
            cache_key=$(printf '%s/%s' "${JOB_NAME:-paas}" '{{ .Key }}' | sha256sum | awk '{print $1}' | cut -c1-16)
            printf '/backup_data/paas-cache/dependencies/%s/{{ .Key }}' "$cache_key"
          ''', returnStdout: true).trim()
          sh "mkdir -p \"$PAAS_DEP_CACHE\""
          withEnv(["PAAS_CACHE_ROOT=/backup_data/paas-cache", "PAAS_DEP_CACHE=${PAAS_DEP_CACHE}"]) {
            docker.image('{{ .BuildImage }}').inside("-v ${PAAS_DEP_CACHE}:${PAAS_DEP_CACHE}") {
              dir('{{ .WorkDir }}') {
                sh '''
                  set -eu
                  mkdir -p "$WORKSPACE/artifact"
                  export PAAS_ARTIFACT_OUTPUT="$WORKSPACE/artifact"
{{ .CollectCommand }}
                  test -n "$(find "$PAAS_ARTIFACT_OUTPUT" -mindepth 1 -maxdepth 1 | head -n 1)"
                '''
              }
            }
          }
        }
      }
    }
{{ end }}
    stage('prepare-image-context') {
      steps {
        writeFile file: 'report/paas-runtime.json', text: '''{{ .RuntimeJSON }}'''
        sh '''
          set -eu
          test -n "$(find artifact -mindepth 1 -maxdepth 1 | head -n 1)"
{{ range .ImageTargets }}
          dockerfile_source="$WORKSPACE/.paas/dockerfiles/{{ .DockerfilePath }}"
          mkdir -p "image-context/{{ .Key }}"
          cp -ar artifact/. "image-context/{{ .Key }}/"
          if [ ! -f "$dockerfile_source" ]; then
            echo "Dockerfile not found: $dockerfile_source" >&2
            exit 1
          fi
          cp "$dockerfile_source" "image-context/{{ .Key }}/Dockerfile"
          : > "image-context/{{ .Key }}/.dockerignore"
{{ end }}
        '''
      }
    }

    stage('init-buildx') {
      steps {
        sh '''
          set -eu
          node_name=$(printf '%s' "${NODE_NAME:-default}" | tr -c 'A-Za-z0-9_.-' '-')
          builder="jenkins-buildx-${node_name}"
          docker buildx inspect "$builder" >/dev/null 2>&1 || docker buildx create --name "$builder" --driver docker-container --use
          docker buildx inspect "$builder" --bootstrap > report/buildx-inspect.txt
          printf 'BUILDX_BUILDER_NAME=%s\n' "$builder" >> report/build-env.sh
        '''
      }
    }

    stage('buildx-push') {
      parallel {
{{ range .ImageTargets }}
        stage('{{ .Key }} image') {
          steps {
            sh '''
              set -eu
              . report/build-env.sh
              image_tag_branch='{{ $.ImageTagBranch }}'
              image_uri='{{ .ImageRepository }}:{{ $.ImageTagDate }}-'"${image_tag_branch}"'-{{ $.ImageTagVersion }}'
              job_name=$(printf '%s' "${JOB_NAME:-paas}" | tr '/ ' '--')
              cache_dir="/backup_data/buildx-cache/${job_name}/{{ .Key }}"
              cache_next="${cache_dir}.next"
              rm -rf "$cache_next"
              mkdir -p "$cache_dir"
              docker buildx build \
                --builder "$BUILDX_BUILDER_NAME" \
                --platform '{{ .Platforms }}' \
                --build-arg 'RUNTIME_BASE={{ .RuntimeBaseImage }}' \
                --build-arg 'ARTIFACT_DEPLOY_PATH={{ .ArtifactDeployPath }}' \
                --cache-from type=local,src="$cache_dir" \
                --cache-to type=local,dest="$cache_next",mode=max \
                -f image-context/{{ .Key }}/Dockerfile \
                -t "$image_uri" \
                --push image-context/{{ .Key }}
              rm -rf "$cache_dir"
              mv "$cache_next" "$cache_dir"
              printf '%s\n' "$image_uri" > report/image-uri-{{ .Key }}.txt
{{ if .IsPrimary }}
              printf '%s\n' "$image_uri" > report/primary-image.txt
{{ end }}
              printf '{{ .EnvKey }}=%s\n' "$image_uri" > report/image-tag-{{ .Key }}.env
            '''
          }
        }
{{ end }}
      }
    }
  }

  post {
    success {
      script {
        if ((env.PAAS_CALLBACK_URL ?: '').trim() && fileExists('report/primary-image.txt')) {
          def commit = fileExists('report/source-{{ .PrimarySourceKey }}-commit.txt') ? readFile('report/source-{{ .PrimarySourceKey }}-commit.txt').trim() : ''
          def artifacts = []
{{ range .ImageTargets }}
          if (fileExists('report/image-uri-{{ .Key }}.txt')) {
            artifacts << [
              source_key: '{{ .SourceKey }}',
              type: 'image',
              name: '{{ .ArtifactName }}',
              uri: readFile('report/image-uri-{{ .Key }}.txt').trim(),
              is_primary: {{ .IsPrimary }},
              selector_labels: {{ .SelectorLabels }},
              metadata: {{ .Metadata }}
            ]
          }
{{ end }}
          writeFile file: 'report/callback-success.json', text: groovy.json.JsonOutput.toJson([status: 'succeeded', commit_sha: commit, artifacts: artifacts])
          sh 'curl -fsS -X POST "$PAAS_CALLBACK_URL" -H "Content-Type: application/json" --data-binary @report/callback-success.json'
        }
      }
    }
    failure {
      script {
        if ((env.PAAS_CALLBACK_URL ?: '').trim()) {
          writeFile file: 'report/callback-failure.json', text: groovy.json.JsonOutput.toJson([status: 'failed', error_message: 'Jenkins build failed'])
          sh 'curl -fsS -X POST "$PAAS_CALLBACK_URL" -H "Content-Type: application/json" --data-binary @report/callback-failure.json'
        }
      }
    }
    always {
      archiveArtifacts artifacts: 'report/**, image-context/**/Dockerfile, image-context/**/.dockerignore', allowEmptyArchive: true
    }
  }
}`

func (s *Service) CreateJenkinsJobTemplate(ctx context.Context, input CreateJenkinsJobTemplateInput) (JenkinsJobTemplate, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "jenkins_template:manage"); err != nil {
		return JenkinsJobTemplate{}, err
	}
	jenkinsfile := firstNonEmpty(input.JenkinsfileContent, input.XMLContent)
	if err := validateJenkinsfile(jenkinsfile); err != nil {
		return JenkinsJobTemplate{}, err
	}
	name := normalizeTemplateName(input.Name)
	if name == "" {
		return JenkinsJobTemplate{}, shared.NewError(shared.CodeInvalidArgument, "template name is required")
	}
	id, err := s.ids.NewID("jenkins_template")
	if err != nil {
		return JenkinsJobTemplate{}, err
	}
	now := s.clock.Now()
	template := JenkinsJobTemplate{ID: id, Name: name, DisplayName: name, Version: 1, XMLContent: strings.TrimSpace(jenkinsfile), Status: JenkinsJobTemplateEnabled, IsDefault: input.IsDefault, CreatedBy: input.Actor.ID, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateJenkinsJobTemplate(ctx, template); err != nil {
		return JenkinsJobTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "jenkins_template.create", ResourceType: "jenkins_job_template", ResourceID: template.ID, Result: "succeeded", Summary: "创建 Jenkins 流水线模板", OccurredAt: now})
	return template, nil
}

func (s *Service) UpdateJenkinsJobTemplate(ctx context.Context, input UpdateJenkinsJobTemplateInput) (JenkinsJobTemplate, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "jenkins_template:manage"); err != nil {
		return JenkinsJobTemplate{}, err
	}
	template, err := s.repo.GetJenkinsJobTemplate(ctx, input.TemplateID)
	if err != nil {
		return JenkinsJobTemplate{}, err
	}
	if input.Status != "" {
		switch input.Status {
		case JenkinsJobTemplateEnabled, JenkinsJobTemplateDisabled:
			template.Status = input.Status
		default:
			return JenkinsJobTemplate{}, shared.NewError(shared.CodeInvalidArgument, "template status is not supported")
		}
	}
	template.IsDefault = input.IsDefault
	template.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateJenkinsJobTemplate(ctx, template); err != nil {
		return JenkinsJobTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "jenkins_template.update", ResourceType: "jenkins_job_template", ResourceID: template.ID, Result: "succeeded", Summary: "更新 Jenkins 流水线模板", OccurredAt: template.UpdatedAt})
	return template, nil
}

func (s *Service) UploadJenkinsJobTemplateRevision(ctx context.Context, input UploadJenkinsJobTemplateRevisionInput) (JenkinsJobTemplate, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "jenkins_template:manage"); err != nil {
		return JenkinsJobTemplate{}, err
	}
	jenkinsfile := firstNonEmpty(input.JenkinsfileContent, input.XMLContent)
	if err := validateJenkinsfile(jenkinsfile); err != nil {
		return JenkinsJobTemplate{}, err
	}
	template, err := s.repo.GetJenkinsJobTemplate(ctx, input.TemplateID)
	if err != nil {
		return JenkinsJobTemplate{}, err
	}
	template.XMLContent = strings.TrimSpace(jenkinsfile)
	template.Version++
	template.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateJenkinsJobTemplate(ctx, template); err != nil {
		return JenkinsJobTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "jenkins_template.revise", ResourceType: "jenkins_job_template", ResourceID: template.ID, Result: "succeeded", Summary: "上传 Jenkins 流水线模板新版本", OccurredAt: template.UpdatedAt})
	return template, nil
}

func (s *Service) DeleteJenkinsJobTemplate(ctx context.Context, actor identityaccess.Subject, templateID shared.ID) error {
	if err := s.checkPlatformAdmin(ctx, actor, "jenkins_template:manage"); err != nil {
		return err
	}
	template, err := s.repo.GetJenkinsJobTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteJenkinsJobTemplate(ctx, templateID); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "jenkins_template.delete", ResourceType: "jenkins_job_template", ResourceID: template.ID, Result: "succeeded", Summary: "删除 Jenkins 构建类型", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) GetJenkinsJobTemplate(ctx context.Context, id shared.ID) (JenkinsJobTemplate, error) {
	return s.repo.GetJenkinsJobTemplate(ctx, id)
}

func (s *Service) ListJenkinsJobTemplates(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[JenkinsJobTemplate], error) {
	return s.repo.ListJenkinsJobTemplates(ctx, includeDisabled, page)
}

func (s *Service) CreateBuildEnvironment(ctx context.Context, input CreateBuildEnvironmentInput) (BuildEnvironment, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "build_environment:manage"); err != nil {
		return BuildEnvironment{}, err
	}
	environment, err := s.newBuildEnvironment(input.Actor.ID, input.Name, input.Description, input.BuildImage, input.IsDefault)
	if err != nil {
		return BuildEnvironment{}, err
	}
	if err := s.repo.CreateBuildEnvironment(ctx, environment); err != nil {
		return BuildEnvironment{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build_environment.create", ResourceType: "build_environment", ResourceID: environment.ID, Result: "succeeded", Summary: "创建构建环境", OccurredAt: environment.CreatedAt})
	return environment, nil
}

func (s *Service) UpdateBuildEnvironment(ctx context.Context, input UpdateBuildEnvironmentInput) (BuildEnvironment, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "build_environment:manage"); err != nil {
		return BuildEnvironment{}, err
	}
	environment, err := s.repo.GetBuildEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return BuildEnvironment{}, err
	}
	if environment.Status == BuildEnvironmentDeleted {
		return BuildEnvironment{}, shared.NewError(shared.CodeNotFound, "build environment not found")
	}
	if input.Status != "" {
		switch input.Status {
		case BuildEnvironmentEnabled, BuildEnvironmentDisabled:
			environment.Status = input.Status
		default:
			return BuildEnvironment{}, shared.NewError(shared.CodeInvalidArgument, "build environment status is not supported")
		}
	}
	environment.Description = strings.TrimSpace(input.Description)
	environment.BuildImage = firstNonEmpty(input.BuildImage, environment.BuildImage)
	environment.IsDefault = input.IsDefault
	environment.UpdatedAt = s.clock.Now()
	if err := validateBuildEnvironment(environment); err != nil {
		return BuildEnvironment{}, err
	}
	if err := s.repo.UpdateBuildEnvironment(ctx, environment); err != nil {
		return BuildEnvironment{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build_environment.update", ResourceType: "build_environment", ResourceID: environment.ID, Result: "succeeded", Summary: "更新构建环境", OccurredAt: environment.UpdatedAt})
	return environment, nil
}

func (s *Service) DeleteBuildEnvironment(ctx context.Context, actor identityaccess.Subject, id shared.ID) error {
	if err := s.checkPlatformAdmin(ctx, actor, "build_environment:manage"); err != nil {
		return err
	}
	if err := s.repo.DeleteBuildEnvironment(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "build_environment.delete", ResourceType: "build_environment", ResourceID: id, Result: "succeeded", Summary: "删除构建环境", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) GetBuildEnvironment(ctx context.Context, id shared.ID) (BuildEnvironment, error) {
	return s.repo.GetBuildEnvironment(ctx, id)
}

func (s *Service) FindDefaultBuildEnvironment(ctx context.Context) (BuildEnvironment, error) {
	return s.repo.FindDefaultBuildEnvironment(ctx)
}

func (s *Service) ListBuildEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[BuildEnvironment], error) {
	return s.repo.ListBuildEnvironments(ctx, includeDisabled, page)
}

func (s *Service) CreateRuntimeEnvironment(ctx context.Context, input CreateRuntimeEnvironmentInput) (RuntimeEnvironment, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "runtime_environment:manage"); err != nil {
		return RuntimeEnvironment{}, err
	}
	environment, err := s.newRuntimeEnvironment(input.Actor.ID, input.Name, input.Description, input.RuntimeBaseImage, input.ArtifactDeployPath, input.DockerfilePath, input.SelectorLabels, input.Images)
	if err != nil {
		return RuntimeEnvironment{}, err
	}
	if err := s.repo.CreateRuntimeEnvironment(ctx, environment); err != nil {
		return RuntimeEnvironment{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "runtime_environment.create", ResourceType: "runtime_environment", ResourceID: environment.ID, Result: "succeeded", Summary: "创建运行时环境", OccurredAt: environment.CreatedAt})
	return environment, nil
}

func (s *Service) UpdateRuntimeEnvironment(ctx context.Context, input UpdateRuntimeEnvironmentInput) (RuntimeEnvironment, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "runtime_environment:manage"); err != nil {
		return RuntimeEnvironment{}, err
	}
	environment, err := s.repo.GetRuntimeEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return RuntimeEnvironment{}, err
	}
	if environment.Status == RuntimeEnvironmentDeleted {
		return RuntimeEnvironment{}, shared.NewError(shared.CodeNotFound, "runtime environment not found")
	}
	if input.Status != "" {
		switch input.Status {
		case RuntimeEnvironmentEnabled, RuntimeEnvironmentDisabled:
			environment.Status = input.Status
		default:
			return RuntimeEnvironment{}, shared.NewError(shared.CodeInvalidArgument, "runtime environment status is not supported")
		}
	}
	environment.Description = strings.TrimSpace(input.Description)
	if input.Images != nil {
		environment.Images = normalizeRuntimeEnvironmentImages(input.Images)
	} else if strings.TrimSpace(input.RuntimeBaseImage) != "" {
		environment.Images = runtimeEnvironmentImagesOrLegacy(nil, environment.ID, environment.Name, input.RuntimeBaseImage, input.ArtifactDeployPath, input.DockerfilePath, input.SelectorLabels)
	}
	applyRuntimeEnvironmentFirstImage(&environment)
	environment.UpdatedAt = s.clock.Now()
	if err := validateRuntimeEnvironment(environment); err != nil {
		return RuntimeEnvironment{}, err
	}
	if err := s.repo.UpdateRuntimeEnvironment(ctx, environment); err != nil {
		return RuntimeEnvironment{}, err
	}
	if s.runtimeSyncer != nil {
		if err := s.runtimeSyncer.SyncRuntimeEnvironment(ctx, input.Actor, environment); err != nil {
			return RuntimeEnvironment{}, err
		}
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "runtime_environment.update", ResourceType: "runtime_environment", ResourceID: environment.ID, Result: "succeeded", Summary: "更新运行时环境", OccurredAt: environment.UpdatedAt})
	return environment, nil
}

func (s *Service) DeleteRuntimeEnvironment(ctx context.Context, actor identityaccess.Subject, id shared.ID) error {
	if err := s.checkPlatformAdmin(ctx, actor, "runtime_environment:manage"); err != nil {
		return err
	}
	if err := s.repo.DeleteRuntimeEnvironment(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "runtime_environment.delete", ResourceType: "runtime_environment", ResourceID: id, Result: "succeeded", Summary: "删除运行时环境", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) GetRuntimeEnvironment(ctx context.Context, id shared.ID) (RuntimeEnvironment, error) {
	return s.repo.GetRuntimeEnvironment(ctx, id)
}

func (s *Service) FindDefaultRuntimeEnvironment(ctx context.Context) (RuntimeEnvironment, error) {
	return s.repo.FindDefaultRuntimeEnvironment(ctx)
}

func (s *Service) ListRuntimeEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[RuntimeEnvironment], error) {
	return s.repo.ListRuntimeEnvironments(ctx, includeDisabled, page)
}

func (s *Service) GetBuildTemplate(ctx context.Context) (BuildTemplate, error) {
	return s.repo.GetBuildTemplate(ctx)
}

func (s *Service) SaveBuildTemplate(ctx context.Context, input SaveBuildTemplateInput) (BuildTemplate, error) {
	if err := s.checkPlatformAdmin(ctx, input.Actor, "build_template:manage"); err != nil {
		return BuildTemplate{}, err
	}
	content := strings.TrimSpace(input.Content)
	if err := validateBuildTemplateContent(content); err != nil {
		return BuildTemplate{}, err
	}
	template, err := s.repo.GetBuildTemplate(ctx)
	if err != nil {
		if shared.CodeOf(err) != shared.CodeNotFound {
			return BuildTemplate{}, err
		}
		template = BuildTemplate{ID: "global-build-template", Name: "global-build-template", CreatedBy: input.Actor.ID, CreatedAt: s.clock.Now()}
	}
	template.Content = content
	template.Version++
	if template.Version <= 0 {
		template.Version = 1
	}
	template.UpdatedAt = s.clock.Now()
	if err := s.repo.SaveBuildTemplate(ctx, template); err != nil {
		return BuildTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build_template.update", ResourceType: "build_template", ResourceID: template.ID, Result: "succeeded", Summary: "更新全局构建模板", OccurredAt: template.UpdatedAt})
	return template, nil
}

func (s *Service) EnsureDefaultBuildConfiguration(ctx context.Context, actorID shared.ID) error {
	if err := s.ensureDefaultBuildEnvironments(ctx, actorID); err != nil {
		return err
	}
	if err := s.ensureDefaultRuntimeEnvironments(ctx, actorID); err != nil {
		return err
	}
	if existing, err := s.repo.GetBuildTemplate(ctx); err == nil {
		if shouldRefreshDefaultBuildTemplate(existing) {
			now := s.clock.Now()
			existing.Version = currentDefaultBuildTemplateVersion
			existing.Content = defaultBuildTemplateContent
			existing.UpdatedAt = now
			if existing.CreatedAt.IsZero() {
				existing.CreatedAt = now
			}
			if existing.CreatedBy.IsZero() {
				existing.CreatedBy = actorID
			}
			return s.repo.SaveBuildTemplate(ctx, existing)
		}
		return nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return err
	}
	now := s.clock.Now()
	return s.repo.SaveBuildTemplate(ctx, BuildTemplate{ID: "global-build-template", Name: "global-build-template", Version: currentDefaultBuildTemplateVersion, Content: defaultBuildTemplateContent, CreatedBy: actorID, CreatedAt: now, UpdatedAt: now})
}

func shouldRefreshDefaultBuildTemplate(template BuildTemplate) bool {
	content := strings.TrimSpace(template.Content)
	if template.ID != "global-build-template" || template.Version >= currentDefaultBuildTemplateVersion || content == "" || content == strings.TrimSpace(defaultBuildTemplateContent) {
		return false
	}
	if template.Version <= 1 && strings.Contains(content, "{{ .ImageURI }}") && !strings.Contains(content, "artifacts: artifacts") {
		return true
	}
	if template.Version <= 2 && strings.Contains(content, "artifacts: artifacts") && strings.Contains(content, "JsonSlurperClassic") {
		return true
	}
	if template.Version <= 3 && strings.Contains(content, "artifacts: artifacts") && strings.Contains(content, "image_tag_commit") && strings.Contains(content, "-{{ .Key }}") && !strings.Contains(content, "ImageTagVersion") {
		return true
	}
	if template.Version <= 4 && strings.Contains(content, "image_tag_commit") && strings.Contains(content, "ImageTagVersion") {
		return true
	}
	if template.Version <= 5 &&
		!strings.Contains(content, defaultSVNCredentialsID) &&
		strings.Contains(content, "checkout {{ .Key }}") &&
		strings.Contains(content, "ImageTagBranch") {
		return true
	}
	return (template.Version <= 6 &&
		!strings.Contains(content, "PAAS_GIT_USER") &&
		strings.Contains(content, "checkout {{ .Key }}") &&
		strings.Contains(content, "git clone --no-checkout")) ||
		(template.Version <= 7 &&
			!strings.Contains(content, "SourceRefB64") &&
			strings.Contains(content, "ref='{{ .SourceRef }}'")) ||
		(template.Version <= 8 &&
			!strings.Contains(content, "SVNCheckoutPaths") &&
			strings.Contains(content, "svn checkout"))
}

func (s *Service) EnsureDefaultJenkinsJobTemplate(ctx context.Context, actorID shared.ID) error {
	if _, err := s.repo.FindDefaultJenkinsJobTemplate(ctx); err == nil {
		return nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return err
	}
	now := s.clock.Now()
	return s.repo.CreateJenkinsJobTemplate(ctx, JenkinsJobTemplate{ID: shared.ID(s.templateID), Name: s.templateID, DisplayName: s.templateID, Version: 1, XMLContent: defaultJenkinsfile, Status: JenkinsJobTemplateEnabled, IsDefault: true, CreatedBy: actorID, CreatedAt: now, UpdatedAt: now})
}

func (s *Service) EnsureBuildPipeline(ctx context.Context, applicationID shared.ID) error {
	return shared.NewError(shared.CodeInvalidArgument, "pipeline_id is required")
}

func (s *Service) DeleteBuildPipeline(ctx context.Context, applicationID shared.ID) error {
	if _, err := s.requireApplication(ctx, applicationID); err != nil {
		return err
	}
	result, err := s.repo.ListPipelinesByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return err
	}
	for _, pipeline := range result.Items {
		if err := s.deletePipeline(ctx, pipeline, shared.ID("system")); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CreateBuildPipeline(ctx context.Context, input CreateBuildPipelineInput) (BuildPipeline, error) {
	app, err := s.requireApplication(ctx, input.ApplicationID)
	if err != nil {
		return BuildPipeline{}, err
	}
	if err := s.check(ctx, input.Actor, app, "build_pipeline:create"); err != nil {
		return BuildPipeline{}, err
	}
	name := normalizePipelineName(input.Name)
	if err := validatePipelineName(name); err != nil {
		return BuildPipeline{}, err
	}
	if _, err := s.repo.FindPipelineByApplicationAndName(ctx, app.ID, name); err == nil {
		return BuildPipeline{}, shared.NewError(shared.CodeConflict, "build pipeline name already exists")
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return BuildPipeline{}, err
	}
	runtimes, err := s.requireEnabledRuntimeEnvironments(ctx, input.RuntimeEnvironmentIDs)
	if err != nil {
		return BuildPipeline{}, err
	}
	sources, err := s.preparePipelineSources(ctx, app, "", input.Sources, runtimes)
	if err != nil {
		return BuildPipeline{}, err
	}
	id, err := s.ids.NewID("build_pipeline")
	if err != nil {
		return BuildPipeline{}, err
	}
	now := s.clock.Now()
	pipeline := BuildPipeline{
		ID:                  id,
		TenantID:            app.TenantID,
		ProjectID:           app.ProjectID,
		ApplicationID:       app.ID,
		Name:                name,
		DisplayName:         normalizeDisplayName(input.DisplayName, name),
		Description:         strings.TrimSpace(input.Description),
		Provider:            "jenkins",
		ExternalJobName:     s.pipelineJobName(app, name),
		TemplateID:          "global-build-template",
		Status:              BuildPipelineStatusActive,
		ManagedByPlatform:   true,
		RuntimeEnvironments: runtimes,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := s.repo.CreatePipeline(ctx, pipeline); err != nil {
		return BuildPipeline{}, err
	}
	for i := range sources {
		sources[i].PipelineID = pipeline.ID
		sources[i].CreatedAt = now
		sources[i].UpdatedAt = now
	}
	if err := s.repo.ReplacePipelineSources(ctx, pipeline.ID, sources); err != nil {
		return BuildPipeline{}, err
	}
	if err := s.repo.ReplacePipelineRuntimeEnvironments(ctx, pipeline.ID, runtimes); err != nil {
		return BuildPipeline{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build_pipeline.create", ResourceType: "build_pipeline", ResourceID: pipeline.ID, Result: "succeeded", Summary: "创建构建流水线", OccurredAt: now})
	pipeline.RuntimeEnvironments = runtimes
	return pipeline, nil
}

func (s *Service) ListBuildPipelines(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildPipeline], error) {
	if _, err := s.requireApplication(ctx, applicationID); err != nil {
		return shared.PageResult[BuildPipeline]{}, err
	}
	return s.repo.ListPipelinesByApplication(ctx, applicationID, page)
}

func (s *Service) GetBuildPipeline(ctx context.Context, pipelineID shared.ID) (BuildPipeline, error) {
	return s.repo.GetPipeline(ctx, pipelineID)
}

func (s *Service) ListBuildPipelineSources(ctx context.Context, pipelineID shared.ID) ([]BuildPipelineSource, error) {
	return s.repo.ListPipelineSources(ctx, pipelineID)
}

func (s *Service) PreviewGitBranches(ctx context.Context, input ListGitBranchesInput) ([]SourceBranch, error) {
	if input.Actor.ID.IsZero() {
		return nil, shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if input.ProjectID.IsZero() {
		return nil, shared.NewError(shared.CodeInvalidArgument, "project_id is required")
	}
	if s.gitSources == nil {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "git source query port is required")
	}
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" {
		return nil, shared.NewError(shared.CodeInvalidArgument, "source_url is required")
	}
	project, err := s.gitSources.ResolveProjectByHTTPURL(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	branches, err := s.gitSources.ListBranches(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].Default != branches[j].Default {
			return branches[i].Default
		}
		return branches[i].Name < branches[j].Name
	})
	return branches, nil
}

func (s *Service) UpdateBuildPipeline(ctx context.Context, input UpdateBuildPipelineInput) (BuildPipeline, error) {
	pipeline, err := s.repo.GetPipeline(ctx, input.PipelineID)
	if err != nil {
		return BuildPipeline{}, err
	}
	app, err := s.requireApplication(ctx, pipeline.ApplicationID)
	if err != nil {
		return BuildPipeline{}, err
	}
	if err := s.check(ctx, input.Actor, app, "build_pipeline:update"); err != nil {
		return BuildPipeline{}, err
	}
	runtimes := pipeline.RuntimeEnvironments
	if len(input.RuntimeEnvironmentIDs) > 0 {
		runtimes, err = s.requireEnabledRuntimeEnvironments(ctx, input.RuntimeEnvironmentIDs)
		if err != nil {
			return BuildPipeline{}, err
		}
	}
	if len(runtimes) == 0 {
		return BuildPipeline{}, shared.NewError(shared.CodeInvalidArgument, "runtime_environment_ids is required")
	}
	if len(input.Sources) > 0 {
		sources, err := s.preparePipelineSources(ctx, app, pipeline.ID, input.Sources, runtimes)
		if err != nil {
			return BuildPipeline{}, err
		}
		now := s.clock.Now()
		for i := range sources {
			sources[i].PipelineID = pipeline.ID
			sources[i].CreatedAt = now
			sources[i].UpdatedAt = now
		}
		if err := s.repo.ReplacePipelineSources(ctx, pipeline.ID, sources); err != nil {
			return BuildPipeline{}, err
		}
	}
	if len(input.RuntimeEnvironmentIDs) > 0 {
		if err := s.repo.ReplacePipelineRuntimeEnvironments(ctx, pipeline.ID, runtimes); err != nil {
			return BuildPipeline{}, err
		}
	}
	pipeline.DisplayName = normalizeDisplayName(input.DisplayName, pipeline.Name)
	pipeline.Description = strings.TrimSpace(input.Description)
	pipeline.RuntimeEnvironments = runtimes
	pipeline.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdatePipeline(ctx, pipeline); err != nil {
		return BuildPipeline{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build_pipeline.update", ResourceType: "build_pipeline", ResourceID: pipeline.ID, Result: "succeeded", Summary: "更新构建流水线", OccurredAt: pipeline.UpdatedAt})
	return pipeline, nil
}

func (s *Service) DeleteNamedBuildPipeline(ctx context.Context, actor identityaccess.Subject, pipelineID shared.ID) error {
	pipeline, err := s.repo.GetPipeline(ctx, pipelineID)
	if err != nil {
		return err
	}
	app, err := s.requireApplication(ctx, pipeline.ApplicationID)
	if err != nil {
		return err
	}
	if err := s.check(ctx, actor, app, "build_pipeline:delete"); err != nil {
		return err
	}
	if err := s.ensurePipelineNotBoundToWorkload(ctx, pipeline); err != nil {
		return err
	}
	return s.deletePipeline(ctx, pipeline, actor.ID)
}

func (s *Service) ensurePipelineNotBoundToWorkload(ctx context.Context, pipeline BuildPipeline) error {
	if s.workloads == nil || pipeline.ID.IsZero() {
		return nil
	}
	workloads, err := s.workloads.ListEnabledWorkloadsByPipeline(ctx, pipeline.ApplicationID, pipeline.ID)
	if err != nil {
		if shared.CodeOf(err) == shared.CodeNotFound {
			return nil
		}
		return err
	}
	for _, workload := range workloads {
		if workload.PipelineID == pipeline.ID || strings.TrimSpace(workload.ContainerName) != "" {
			return shared.NewError(shared.CodeFailedPrecondition, "已有工作负载关联，不能删除")
		}
	}
	return nil
}

func (s *Service) deletePipeline(ctx context.Context, pipeline BuildPipeline, actorID shared.ID) error {
	active, err := s.repo.HasActiveRunsByPipeline(ctx, pipeline.ID)
	if err != nil {
		return err
	}
	if active {
		return shared.NewError(shared.CodeFailedPrecondition, "build pipeline has active build runs")
	}
	if pipeline.ManagedByPlatform && s.runner != nil && strings.TrimSpace(pipeline.ExternalJobName) != "" {
		if err := s.runner.DeleteJob(ctx, pipeline.ExternalJobName); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return err
		}
	}
	pipeline.Status = BuildPipelineStatusDisabled
	pipeline.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdatePipeline(ctx, pipeline); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: "build_pipeline.delete", ResourceType: "build_pipeline", ResourceID: pipeline.ID, Result: "succeeded", Summary: "删除 Jenkins 构建任务", OccurredAt: pipeline.UpdatedAt})
	return nil
}

func (s *Service) TriggerBuild(ctx context.Context, input TriggerBuildInput) (BuildRun, error) {
	app, pipeline, sources, err := s.loadPipelineBuildContext(ctx, input.PipelineID)
	if err != nil {
		return BuildRun{}, err
	}
	if err := s.check(ctx, input.Actor, app, "build:create"); err != nil {
		return BuildRun{}, err
	}
	for _, source := range sources {
		if err := validateBuildSpec(source.BuildSpec); err != nil {
			return BuildRun{}, err
		}
	}
	if err := validateBuildVersion(input.Version); err != nil {
		return BuildRun{}, err
	}
	runID, err := s.ids.NewID("build_run")
	if err != nil {
		return BuildRun{}, err
	}
	now := s.clock.Now()
	overrides := buildSourceOverrides(input.normalizedSourceOverrides(sources[0].Key))
	runSources, err := s.newBuildRunSources(ctx, runID, app, sources, overrides, now)
	if err != nil {
		return BuildRun{}, err
	}
	primary := runSources[0]
	run := BuildRun{
		ID:                  runID,
		TenantID:            app.TenantID,
		ProjectID:           app.ProjectID,
		ApplicationID:       app.ID,
		PipelineName:        pipeline.Name,
		PipelineDisplayName: pipeline.DisplayName,
		SourceType:          primary.SourceType,
		SourceURL:           primary.SourceURL,
		SourceRef:           primary.SourceRef,
		CommitSHA:           primary.CommitSHA,
		Version:             normalizeBuildVersion(input.Version),
		Status:              BuildRunQueued,
		RequestedBy:         input.Actor.ID,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	pipeline, err = s.ensurePipeline(ctx, app, pipeline, sources, runSources, run)
	if err != nil {
		return BuildRun{}, err
	}
	run.PipelineID = pipeline.ID
	if err := s.repo.CreateRunWithSources(ctx, run, runSources); err != nil {
		return BuildRun{}, err
	}
	queue, err := s.runnerOrError().TriggerBuild(ctx, pipeline.ExternalJobName, map[string]string{})
	if err != nil {
		now := s.clock.Now()
		run.Status = BuildRunFailed
		run.ErrorMessage = "Jenkins 构建触发失败"
		run.FinishedAt = &now
		run.UpdatedAt = now
		_ = s.repo.UpdateRun(ctx, run)
		return BuildRun{}, err
	}
	run.JenkinsQueueID = strings.TrimSpace(queue.QueueID)
	if queue.BuildNumber > 0 {
		run.JenkinsBuildNumber = queue.BuildNumber
		run.Status = BuildRunRunning
		run.StartedAt = &now
	}
	run.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return BuildRun{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "build.trigger", ResourceType: "build_run", ResourceID: run.ID, Result: "succeeded", Summary: "触发应用构建", Details: buildSpecAuditDetails(sources[0].BuildSpec), OccurredAt: now})
	_ = s.publish(ctx, "BuildStarted", now, BuildStartedPayload{BuildRunID: run.ID, ApplicationID: app.ID, ProjectID: app.ProjectID})
	return run, nil
}

func (s *Service) SyncQueueItem(ctx context.Context, buildRunID shared.ID) (BuildRun, error) {
	run, err := s.repo.GetRun(ctx, buildRunID)
	if err != nil {
		return BuildRun{}, err
	}
	if run.JenkinsQueueID == "" || run.JenkinsBuildNumber > 0 {
		return run, nil
	}
	queue, err := s.runnerOrError().GetQueueItem(ctx, run.JenkinsQueueID)
	if err != nil {
		return BuildRun{}, err
	}
	now := s.clock.Now()
	if queue.Canceled {
		run.Status = BuildRunAborted
		run.FinishedAt = &now
	} else if queue.BuildNumber > 0 {
		run.JenkinsBuildNumber = queue.BuildNumber
		run.Status = BuildRunRunning
		run.StartedAt = &now
	}
	run.UpdatedAt = now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return BuildRun{}, err
	}
	return run, nil
}

func (s *Service) refreshActiveRunsByPipeline(ctx context.Context, pipeline BuildPipeline) error {
	runs, err := s.repo.ListActiveRunsByPipeline(ctx, pipeline.ID)
	if err != nil {
		return err
	}
	for _, run := range runs {
		current := run
		if current.JenkinsBuildNumber == 0 && current.JenkinsQueueID != "" {
			synced, err := s.SyncQueueItem(ctx, current.ID)
			if err != nil {
				return err
			}
			current = synced
		}
		if current.JenkinsBuildNumber > 0 && !terminalStatus(current.Status) {
			if err := s.refreshBuildStatus(ctx, pipeline, current); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) refreshBuildStatus(ctx context.Context, pipeline BuildPipeline, run BuildRun) error {
	status, err := s.runnerOrError().GetBuildStatus(ctx, pipeline.ExternalJobName, run.JenkinsBuildNumber)
	if err != nil {
		return err
	}
	if status.BuildNumber > 0 {
		run.JenkinsBuildNumber = status.BuildNumber
	}
	if status.Building {
		if run.Status == BuildRunRunning {
			return nil
		}
		_, err := s.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunRunning, JenkinsBuildNumber: run.JenkinsBuildNumber})
		return err
	}
	if !terminalStatus(status.Status) {
		return nil
	}
	message := ""
	if status.Status != BuildRunSucceeded {
		message = "Jenkins 构建已结束但 PaaS 未收到回调"
	}
	_, err = s.HandleBuildCallback(ctx, BuildCallbackInput{
		BuildRunID:         run.ID,
		Status:             status.Status,
		JenkinsBuildNumber: run.JenkinsBuildNumber,
		ErrorMessage:       message,
	})
	return err
}

func (s *Service) CancelBuild(ctx context.Context, actor identityaccess.Subject, buildRunID shared.ID) (BuildRun, error) {
	run, err := s.repo.GetRun(ctx, buildRunID)
	if err != nil {
		return BuildRun{}, err
	}
	app, err := s.requireApplication(ctx, run.ApplicationID)
	if err != nil {
		return BuildRun{}, err
	}
	if err := s.check(ctx, actor, app, "build:cancel"); err != nil {
		return BuildRun{}, err
	}
	if terminalStatus(run.Status) {
		return run, nil
	}
	pipeline, err := s.repo.GetPipeline(ctx, run.PipelineID)
	if err != nil {
		return BuildRun{}, err
	}
	runner := s.runnerOrError()
	now := s.clock.Now()
	if run.JenkinsBuildNumber == 0 && run.JenkinsQueueID != "" {
		queue, err := runner.GetQueueItem(ctx, run.JenkinsQueueID)
		if err != nil {
			return BuildRun{}, err
		}
		switch {
		case queue.Canceled:
			run.Status = BuildRunAborted
			run.FinishedAt = &now
		case queue.BuildNumber > 0:
			run.JenkinsBuildNumber = queue.BuildNumber
			run.Status = BuildRunRunning
			if run.StartedAt == nil {
				run.StartedAt = &now
			}
		default:
			if err := runner.CancelQueueItem(ctx, run.JenkinsQueueID); err != nil {
				return BuildRun{}, err
			}
		}
	}
	if run.JenkinsBuildNumber > 0 {
		if err := runner.CancelBuild(ctx, pipeline.ExternalJobName, run.JenkinsBuildNumber); err != nil {
			return BuildRun{}, err
		}
	}
	run.Status = BuildRunAborted
	run.FinishedAt = &now
	run.UpdatedAt = now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return BuildRun{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "build.cancel", ResourceType: "build_run", ResourceID: run.ID, Result: "succeeded", Summary: "取消应用构建", OccurredAt: now})
	return run, nil
}

func (s *Service) GetBuildRun(ctx context.Context, id shared.ID) (BuildRun, error) {
	return s.repo.GetRun(ctx, id)
}

func (s *Service) ListBuildRuns(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRun], error) {
	if _, err := s.requireApplication(ctx, applicationID); err != nil {
		return shared.PageResult[BuildRun]{}, err
	}
	s.refreshActiveRunsByApplication(ctx, applicationID)
	return s.repo.ListRunsByApplication(ctx, applicationID, page)
}

func (s *Service) refreshActiveRunsByApplication(ctx context.Context, applicationID shared.ID) {
	pipelines, err := s.repo.ListPipelinesByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 500})
	if err != nil {
		return
	}
	for _, pipeline := range pipelines.Items {
		_ = s.refreshActiveRunsByPipeline(ctx, pipeline)
	}
}

func (s *Service) ListBuildArtifacts(ctx context.Context, buildRunID shared.ID) ([]BuildArtifact, error) {
	if _, err := s.repo.GetRun(ctx, buildRunID); err != nil {
		return nil, err
	}
	return s.repo.ListArtifactsByRun(ctx, buildRunID)
}

func (s *Service) ListBuildRunSources(ctx context.Context, buildRunID shared.ID) ([]BuildRunSource, error) {
	if _, err := s.repo.GetRun(ctx, buildRunID); err != nil {
		return nil, err
	}
	return s.repo.ListRunSources(ctx, buildRunID)
}

func (s *Service) StreamBuildLogs(ctx context.Context, buildRunID shared.ID) ([]LogEvent, error) {
	return s.streamBuildLogDelta(ctx, buildRunID)
}

func (s *Service) BuildLogEvents(ctx context.Context, buildRunID shared.ID) ([]LogEvent, error) {
	logs, err := s.repo.ListBuildLogs(ctx, buildRunID)
	if err != nil {
		return nil, err
	}
	events := make([]LogEvent, 0, len(logs)+1)
	for _, log := range logs {
		if log != "" {
			events = append(events, LogEvent{Event: "log", Data: log})
		}
	}
	delta, err := s.streamBuildLogDelta(ctx, buildRunID)
	if err != nil {
		return nil, err
	}
	return append(events, delta...), nil
}

func (s *Service) streamBuildLogDelta(ctx context.Context, buildRunID shared.ID) ([]LogEvent, error) {
	run, err := s.repo.GetRun(ctx, buildRunID)
	if err != nil {
		return nil, err
	}
	if run.JenkinsBuildNumber == 0 {
		if run.JenkinsQueueID != "" {
			synced, syncErr := s.SyncQueueItem(ctx, buildRunID)
			if syncErr != nil {
				return nil, syncErr
			}
			run = synced
		}
		if run.JenkinsBuildNumber == 0 {
			return []LogEvent{{Event: "status", Data: string(run.Status)}}, nil
		}
	}
	pipeline, err := s.repo.GetPipeline(ctx, run.PipelineID)
	if err != nil {
		return nil, err
	}
	result, err := s.drainBuildLog(ctx, run, pipeline, maxProgressiveLogDrainIterations)
	if err != nil {
		return nil, err
	}
	events := result.events
	if terminalStatus(result.run.Status) && result.complete {
		events = append(events, LogEvent{Event: "status", Data: string(result.run.Status)})
	}
	return events, nil
}

func (s *Service) drainBuildLog(ctx context.Context, run BuildRun, pipeline BuildPipeline, maxIterations int) (buildLogDrainResult, error) {
	result := buildLogDrainResult{run: run, events: make([]LogEvent, 0), complete: true}
	if run.JenkinsBuildNumber == 0 {
		return result, nil
	}
	if maxIterations <= 0 {
		maxIterations = 1
	}
	for i := 0; i < maxIterations; i++ {
		text, err := s.runnerOrError().ProgressiveText(ctx, pipeline.ExternalJobName, result.run.JenkinsBuildNumber, result.run.LogOffset)
		if err != nil {
			return result, err
		}
		if text.Text == "" && text.NextOffset == 0 {
			text.NextOffset = result.run.LogOffset
		}
		if text.Text != "" && text.NextOffset == 0 && result.run.LogOffset == 0 {
			text.NextOffset = int64(len(text.Text))
		}
		if text.NextOffset < result.run.LogOffset {
			return result, shared.NewError(shared.CodeFailedPrecondition, "jenkins log offset moved backwards")
		}
		if text.Text != "" && text.NextOffset == result.run.LogOffset {
			result.complete = !text.MoreData
			return result, nil
		}
		if text.Text != "" {
			redacted := s.RedactLog(text.Text)
			if err := s.repo.AppendBuildLog(ctx, result.run.ID, redacted); err != nil {
				return result, err
			}
			result.events = append(result.events, LogEvent{Event: "log", Data: redacted})
		}
		advanced := text.NextOffset > result.run.LogOffset
		if advanced {
			result.run.LogOffset = text.NextOffset
			result.run.UpdatedAt = s.clock.Now()
			if err := s.repo.UpdateRun(ctx, result.run); err != nil {
				return result, err
			}
		}
		if !text.MoreData {
			result.complete = true
			return result, nil
		}
		result.complete = false
		if !advanced {
			return result, nil
		}
	}
	result.complete = false
	return result, nil
}

func (s *Service) HandleBuildCallback(ctx context.Context, input BuildCallbackInput) (BuildRun, error) {
	run, err := s.repo.GetRun(ctx, input.BuildRunID)
	if err != nil {
		return BuildRun{}, err
	}
	if terminalStatus(run.Status) {
		return run, nil
	}
	if err := shared.ValidateStatus(string(input.Status), AllowedBuildRunStatuses); err != nil {
		return BuildRun{}, err
	}
	now := s.clock.Now()
	run.Status = input.Status
	run.ErrorMessage = strings.TrimSpace(input.ErrorMessage)
	run.UpdatedAt = now
	if input.JenkinsBuildNumber > 0 {
		run.JenkinsBuildNumber = input.JenkinsBuildNumber
	}
	if strings.TrimSpace(input.CommitSHA) != "" {
		run.CommitSHA = strings.TrimSpace(input.CommitSHA)
	}
	if input.Status == BuildRunRunning {
		run.StartedAt = &now
	} else if terminalStatus(input.Status) {
		run.FinishedAt = &now
	}
	if input.Status == BuildRunSucceeded {
		artifacts, err := s.createBuildArtifacts(ctx, run, input)
		if err != nil {
			return BuildRun{}, err
		}
		for _, artifact := range artifacts {
			if artifact.IsPrimary {
				run.PrimaryArtifactID = artifact.ID
				break
			}
		}
		if run.PrimaryArtifactID.IsZero() && len(artifacts) > 0 {
			run.PrimaryArtifactID = artifacts[0].ID
		}
	}
	if terminalStatus(input.Status) && run.JenkinsBuildNumber > 0 {
		if pipeline, err := s.repo.GetPipeline(ctx, run.PipelineID); err == nil {
			if result, drainErr := s.drainBuildLog(ctx, run, pipeline, maxProgressiveLogDrainIterations); drainErr == nil {
				run = result.run
			}
		}
	}
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		return BuildRun{}, err
	}
	if input.Status == BuildRunSucceeded {
		artifacts, _ := s.repo.ListArtifactsByRun(ctx, run.ID)
		artifactIDs := make([]shared.ID, 0, len(artifacts))
		for _, artifact := range artifacts {
			artifactIDs = append(artifactIDs, artifact.ID)
		}
		workloadTargets, _ := s.pipelineBoundWorkloadTargets(ctx, run.ApplicationID, run.PipelineID)
		workloadIDs := workloadIDsFromTargets(workloadTargets)
		if err := s.publish(ctx, "BuildSucceeded", now, BuildSucceededPayload{BuildRunID: run.ID, ApplicationID: run.ApplicationID, WorkloadID: run.WorkloadID, WorkloadIDs: workloadIDs, WorkloadTargets: workloadTargets, PipelineID: run.PipelineID, PipelineName: run.PipelineName, PipelineDisplayName: run.PipelineDisplayName, BuildArtifactID: run.PrimaryArtifactID, BuildArtifactIDs: artifactIDs, CommitSHA: run.CommitSHA}); err != nil {
			run.ErrorMessage = "BuildSucceeded event publish failed: " + strings.TrimSpace(err.Error())
			run.UpdatedAt = s.clock.Now()
			_ = s.repo.UpdateRun(ctx, run)
		}
	} else if terminalStatus(input.Status) {
		_ = s.publish(ctx, "BuildFailed", now, BuildFailedPayload{BuildRunID: run.ID, ApplicationID: run.ApplicationID, Status: string(run.Status), Message: run.ErrorMessage})
	}
	return run, nil
}

func (s *Service) RedactLog(text string) string {
	redacted := text
	for _, value := range s.sensitiveValues {
		redacted = strings.ReplaceAll(redacted, value, "******")
	}
	for _, marker := range []string{"PAAS_TOKEN=", "GITLAB_TOKEN=", "REGISTRY_PASSWORD="} {
		redacted = redactAssignment(redacted, marker)
	}
	return redacted
}

func (s *Service) pipelineBoundWorkloadIDs(ctx context.Context, applicationID shared.ID, pipelineID shared.ID) ([]shared.ID, error) {
	targets, err := s.pipelineBoundWorkloadTargets(ctx, applicationID, pipelineID)
	if err != nil {
		return nil, err
	}
	return workloadIDsFromTargets(targets), nil
}

func (s *Service) pipelineBoundWorkloadTargets(ctx context.Context, applicationID shared.ID, pipelineID shared.ID) ([]WorkloadTarget, error) {
	if s.workloads == nil || pipelineID.IsZero() {
		return nil, nil
	}
	workloads, err := s.workloads.ListEnabledWorkloadsByPipeline(ctx, applicationID, pipelineID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkloadTarget, 0, len(workloads))
	for _, workload := range workloads {
		if workload.ID != "" {
			out = append(out, WorkloadTarget{WorkloadID: workload.ID, ContainerName: normalizeContainerName(workload.ContainerName)})
		}
	}
	return out, nil
}

func workloadIDsFromTargets(targets []WorkloadTarget) []shared.ID {
	out := make([]shared.ID, 0, len(targets))
	seen := map[shared.ID]struct{}{}
	for _, target := range targets {
		if target.WorkloadID.IsZero() {
			continue
		}
		if _, ok := seen[target.WorkloadID]; ok {
			continue
		}
		seen[target.WorkloadID] = struct{}{}
		out = append(out, target.WorkloadID)
	}
	return out
}

func (s *Service) ensurePipeline(ctx context.Context, app ApplicationRef, pipeline BuildPipeline, sources []ApplicationSourceRef, runSources []BuildRunSource, run BuildRun) (BuildPipeline, error) {
	if len(sources) == 0 {
		return BuildPipeline{}, shared.NewError(shared.CodeNotFound, "build pipeline source not found")
	}
	template, err := s.repo.GetBuildTemplate(ctx)
	if err != nil {
		if shared.CodeOf(err) != shared.CodeNotFound {
			return BuildPipeline{}, err
		}
		template = BuildTemplate{ID: "global-build-template", Name: "global-build-template", Version: 1, Content: defaultBuildTemplateContent}
	}
	jenkinsfile, err := s.renderBuildTemplate(ctx, template.Content, app, pipeline, sources, runSources, run)
	if err != nil {
		return BuildPipeline{}, err
	}
	pipeline.ExternalJobName = s.pipelineJobName(app, pipeline.Name)
	pipeline.TemplateID = template.ID.String()
	pipeline.ConfigHash = ""
	pipeline.UpdatedAt = s.clock.Now()
	if err := s.runnerOrError().EnsureJob(ctx, BuildJobSpec{JobName: pipeline.ExternalJobName, TemplateID: pipeline.TemplateID, TemplateXML: jenkinsPipelineJobXML(jenkinsfile)}); err != nil {
		return BuildPipeline{}, err
	}
	if err := s.repo.UpdatePipeline(ctx, pipeline); err != nil {
		return BuildPipeline{}, err
	}
	return pipeline, nil
}

type buildTemplateView struct {
	AgentLabel           string
	Sources              []buildTemplateSourceView
	Runtime              map[string]string
	DockerfileRepository buildTemplateDockerfileRepositoryView
	DockerfilePath       string
	ArtifactDeployPath   string
	RuntimeBaseImage     string
	CallbackURL          string
	RuntimeJSON          string
	PrimarySourceKey     string
	ImageTargets         []buildTemplateImageTargetView
	ImageTagDate         string
	ImageTagBranch       string
	ImageTagVersion      string
}

type buildTemplateDockerfileRepositoryView struct {
	URL           string
	Ref           string
	CredentialsID string
}

type buildTemplateSourceView struct {
	Key              string
	StageName        string
	SourceType       string
	RepoURL          string
	SourceRef        string
	SourceRefB64     string
	GitCredentials   string
	SVNRevision      string
	SVNCredentials   string
	SVNCheckoutPaths []buildTemplateSVNCheckoutPathView
	CheckoutDir      string
	WorkDir          string
	BuildImage       string
	BuildCommand     string
	CollectCommand   string
}

type buildTemplateSVNCheckoutPathView struct {
	Local string
	Path  string
	Depth string
}

type buildTemplateImageTargetView struct {
	Key                string
	StageName          string
	Platforms          string
	RuntimeBaseImage   string
	DockerfilePath     string
	ArtifactDeployPath string
	ImageRepository    string
	EnvKey             string
	SourceKey          string
	ArtifactName       string
	SelectorLabels     string
	Metadata           string
	IsPrimary          bool
}

func (s *Service) renderBuildTemplate(ctx context.Context, content string, app ApplicationRef, pipeline BuildPipeline, sources []ApplicationSourceRef, runSources []BuildRunSource, run BuildRun) (string, error) {
	if strings.TrimSpace(content) == "" {
		content = defaultBuildTemplateContent
	}
	tpl, err := template.New("build-template").Parse(content)
	if err != nil {
		return "", shared.WrapError(shared.CodeInvalidArgument, "build template syntax is invalid", err)
	}
	runtimePayload := buildRuntimePayload(pipeline.RuntimeEnvironments, sources)
	imageTargets := s.buildTemplateImageTargets(app, pipeline, pipeline.RuntimeEnvironments, sources, runSources, run)
	imageTagDate, imageTagBranch := buildImageTagParts(runSources, run)
	view := buildTemplateView{
		AgentLabel:           "any",
		Sources:              make([]buildTemplateSourceView, 0, len(sources)),
		Runtime:              map[string]string{},
		DockerfileRepository: s.buildTemplateDockerfileRepositoryView(),
		CallbackURL:          groovySingleQuoted(s.buildCallbackURL(run)),
		RuntimeJSON:          groovyTripleSingleQuotedJSON(runtimePayload),
		PrimarySourceKey:     shellSingleQuoted(primaryBuildSourceKey(sources, runSources)),
		ImageTargets:         imageTargets,
		ImageTagDate:         shellSingleQuoted(imageTagDate),
		ImageTagBranch:       shellSingleQuoted(imageTagBranch),
		ImageTagVersion:      shellSingleQuoted(buildVersionTag(run.Version)),
	}
	if len(imageTargets) > 0 {
		view.DockerfilePath = imageTargets[0].DockerfilePath
		view.ArtifactDeployPath = imageTargets[0].ArtifactDeployPath
		view.RuntimeBaseImage = imageTargets[0].RuntimeBaseImage
	}
	if runtimes := runtimePayload; len(runtimes) > 0 {
		view.Runtime = map[string]string{}
		for key, value := range runtimes[0] {
			view.Runtime[key] = value
		}
		view.Runtime["application_name"] = app.Name
	} else if len(sources) > 0 {
		spec := sources[0].BuildSpec
		view.Runtime = map[string]string{
			"runtime_base_image":   spec.RuntimeBaseImage,
			"artifact_deploy_path": spec.ArtifactDeployPath,
			"application_name":     app.Name,
		}
	}
	runSourceByKey := map[string]BuildRunSource{}
	for _, runSource := range runSources {
		runSourceByKey[runSource.SourceKey] = runSource
	}
	for _, source := range sources {
		spec := source.BuildSpec
		buildImage := defaultBuildImage()
		if source.BuildEnvironmentID != "" {
			if environment, err := s.repo.GetBuildEnvironment(ctx, source.BuildEnvironmentID); err == nil {
				if strings.TrimSpace(environment.BuildImage) != "" {
					buildImage = strings.TrimSpace(environment.BuildImage)
				}
			}
		}
		checkoutDir := "source/" + source.Key
		workDir := checkoutDir
		if spec.SourcePath != "." {
			workDir += "/" + spec.SourcePath
		}
		collectCommand := normalizeShellBlock(spec.ArtifactCopyCommand)
		sourceType := normalizeSourceType(source.SourceType)
		sourceRef := firstNonEmpty(source.SourceRef, spec.DefaultRef, defaultSourceRef(sourceType, source.SourceURL))
		if runSource, ok := runSourceByKey[source.Key]; ok && strings.TrimSpace(runSource.SourceRef) != "" {
			sourceRef = runSource.SourceRef
		}
		svnCheckoutPaths := source.SVNCheckoutPaths
		if runSource, ok := runSourceByKey[source.Key]; ok && len(runSource.SVNCheckoutPaths) > 0 {
			svnCheckoutPaths = runSource.SVNCheckoutPaths
		}
		view.Sources = append(view.Sources, buildTemplateSourceView{
			Key:              shellToken(source.Key),
			StageName:        groovyStageName(firstNonEmpty(source.DisplayName, source.Key)),
			SourceType:       string(sourceType),
			RepoURL:          shellSingleQuoted(source.SourceURL),
			SourceRef:        shellSingleQuoted(sourceRef),
			SourceRefB64:     base64.StdEncoding.EncodeToString([]byte(sourceRef)),
			GitCredentials:   groovySingleQuoted(defaultGitCredentialsID),
			SVNRevision:      shellSingleQuoted(firstNonEmpty(source.SVNRevision, "HEAD")),
			SVNCredentials:   groovySingleQuoted(defaultSVNCredentialsID),
			SVNCheckoutPaths: buildTemplateSVNCheckoutPaths(svnCheckoutPaths),
			CheckoutDir:      groovySingleQuoted(checkoutDir),
			WorkDir:          groovySingleQuoted(workDir),
			BuildImage:       groovySingleQuoted(buildImage),
			BuildCommand:     normalizeShellBlock(spec.BuildCommand),
			CollectCommand:   collectCommand,
		})
	}
	var b strings.Builder
	if err := tpl.Execute(&b, view); err != nil {
		return "", shared.NewError(shared.CodeInvalidArgument, "build template render failed: "+err.Error())
	}
	return b.String(), nil
}

func (s *Service) buildTemplateDockerfileRepositoryView() buildTemplateDockerfileRepositoryView {
	return buildTemplateDockerfileRepositoryView{
		URL:           shellSingleQuoted(s.dockerfileRepo.URL),
		Ref:           shellSingleQuoted(firstNonEmpty(s.dockerfileRepo.Ref, "main")),
		CredentialsID: groovySingleQuoted(s.dockerfileRepo.CredentialsID),
	}
}

func (s *Service) buildTemplateImageTargets(app ApplicationRef, pipeline BuildPipeline, runtimes []RuntimeEnvironmentRef, sources []ApplicationSourceRef, runSources []BuildRunSource, run BuildRun) []buildTemplateImageTargetView {
	imageRepo := strings.TrimRight(strings.TrimSpace(s.imageRepository), "/")
	if imageRepo == "" {
		imageRepo = "registry.local/paas"
	}
	primarySource := primaryBuildSource(sources, runSources)
	sourceKey := primaryBuildSourceKey(sources, runSources)
	runtimes = nonEmptyRuntimeEnvironments(runtimes)
	if len(runtimes) == 0 && len(sources) > 0 {
		runtimes = []RuntimeEnvironmentRef{{
			Name:               firstNonEmpty(primarySource.Key, "main"),
			RuntimeBaseImage:   primarySource.BuildSpec.RuntimeBaseImage,
			ArtifactDeployPath: primarySource.BuildSpec.ArtifactDeployPath,
		}}
	}
	targets := make([]buildTemplateImageTargetView, 0, len(runtimes))
	targetIndex := 0
	for _, runtime := range runtimes {
		images := runtimeEnvironmentImagesOrLegacy(runtime.Images, runtime.ID, runtime.Name, runtime.RuntimeBaseImage, runtime.ArtifactDeployPath, runtime.DockerfilePath, runtime.SelectorLabels)
		for _, image := range images {
			name := firstNonEmpty(image.DisplayName, image.Name, runtime.Name)
			key := shellToken(firstNonEmpty(runtime.Name+"-"+image.Name, image.Name, runtime.Name))
			if key == "" {
				key = fmt.Sprintf("runtime%d", targetIndex+1)
			}
			platforms := runtimeTargetPlatforms(image.Architectures)
			imageRepository := buildTargetImageRepository(imageRepo, app.Name, image.Name)
			metadata := map[string]string{
				"runtime_environment_id":         runtime.ID.String(),
				"runtime_environment_name":       runtime.Name,
				"runtime_environment_image_id":   image.ID.String(),
				"runtime_environment_image_name": image.Name,
				"runtime_base_image":             image.RuntimeBaseImage,
				"image_repository":               imageRepository,
				"architectures":                  strings.Join(normalizeImageArchitectures(image.Architectures), ","),
			}
			dockerfilePath := "java/jar/Dockerfile"
			artifactDeployPath := "/app"
			if runtimeImageLooksLikeTomcat(runtime, image) {
				dockerfilePath = "java/tomcat/Dockerfile"
				artifactDeployPath = firstNonEmpty(image.ArtifactDeployPath, runtime.ArtifactDeployPath, primarySource.BuildSpec.ArtifactDeployPath, "/usr/local/tomcat/webapps")
			}
			dockerfilePath = firstNonEmpty(image.DockerfilePath, runtime.DockerfilePath, dockerfilePath)
			targets = append(targets, buildTemplateImageTargetView{
				Key:                shellToken(key),
				StageName:          groovyStageName(name),
				Platforms:          shellSingleQuoted(platforms),
				RuntimeBaseImage:   shellSingleQuoted(image.RuntimeBaseImage),
				DockerfilePath:     shellSingleQuoted(dockerfilePath),
				ArtifactDeployPath: shellSingleQuoted(strings.TrimRight(artifactDeployPath, "/")),
				ImageRepository:    shellSingleQuoted(imageRepository),
				EnvKey:             shellEnvName(key) + "_IMAGE",
				SourceKey:          groovySingleQuoted(sourceKey),
				ArtifactName:       groovySingleQuoted(name),
				SelectorLabels:     groovyStringMapLiteral(normalizeSelectorLabels(image.SelectorLabels)),
				Metadata:           groovyStringMapLiteral(metadata),
				IsPrimary:          targetIndex == 0,
			})
			targetIndex++
		}
	}
	return targets
}

func defaultImageArchitectures() []string {
	return []string{"x86", "arm"}
}

func normalizeImageArchitectures(values []string) []string {
	if len(values) == 0 {
		return defaultImageArchitectures()
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "amd64", "linux/amd64":
			normalized = "x86"
		case "arm64", "linux/arm64":
			normalized = "arm"
		}
		if normalized != "x86" && normalized != "arm" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return defaultImageArchitectures()
	}
	return out
}

func buildTargetImageRepository(basePrefix string, appName string, imageName string) string {
	basePrefix = strings.TrimRight(strings.TrimSpace(basePrefix), "/")
	name := imageTagToken(appName)
	if name == "" {
		name = "app"
	}
	if imageSuffix := imageTagToken(imageName); imageSuffix != "" {
		name += "-" + imageSuffix
	}
	return basePrefix + "/" + name
}

func primaryBuildSource(sources []ApplicationSourceRef, runSources []BuildRunSource) ApplicationSourceRef {
	key := primaryBuildSourceKey(sources, runSources)
	for _, source := range sources {
		if source.Key == key {
			return source
		}
	}
	if len(sources) > 0 {
		return sources[0]
	}
	return ApplicationSourceRef{}
}

func runtimeLooksLikeTomcat(runtime RuntimeEnvironmentRef) bool {
	text := strings.ToLower(runtime.Name + " " + runtime.RuntimeBaseImage + " " + runtime.DockerfilePath)
	return strings.Contains(text, "tomcat")
}

func runtimeImageLooksLikeTomcat(runtime RuntimeEnvironmentRef, image RuntimeEnvironmentImage) bool {
	text := strings.ToLower(runtime.Name + " " + image.Name + " " + image.DisplayName + " " + image.RuntimeBaseImage + " " + image.DockerfilePath)
	return strings.Contains(text, "tomcat")
}

func buildTargetImageURI(imageRepo, appName, baseTag string) string {
	return fmt.Sprintf("%s/%s:%s", imageRepo, appName, baseTag)
}

func runtimeTargetPlatforms(architectures []string) string {
	architectures = normalizeImageArchitectures(architectures)
	platforms := make([]string, 0, len(architectures))
	for _, architecture := range architectures {
		switch architecture {
		case "x86":
			platforms = append(platforms, "linux/amd64")
		case "arm":
			platforms = append(platforms, "linux/arm64")
		}
	}
	if len(platforms) == 0 {
		platforms = []string{"linux/amd64", "linux/arm64"}
	}
	return strings.Join(platforms, ",")
}

func buildImageBaseTag(runSources []BuildRunSource, run BuildRun) string {
	date, branch := buildImageTagParts(runSources, run)
	return date + "-" + branch + "-" + buildVersionTag(run.Version)
}

func buildImageTagParts(runSources []BuildRunSource, run BuildRun) (string, string) {
	source := primaryRunSource(runSources)
	ref := firstNonEmpty(source.SourceRef, run.SourceRef)
	if normalizeSourceType(firstNonEmptySourceType(source.SourceType, run.SourceType)) == SourceTypeSVN {
		ref = firstNonEmpty(svnTagRefFromURL(firstNonEmpty(source.SourceURL, run.SourceURL)), ref)
	}
	branch := imageTagToken(ref)
	date := run.CreatedAt
	if date.IsZero() {
		date = time.Now()
	}
	return date.Format("20060102"), branch
}

func imageTagToken(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			b.WriteRune(r)
			continue
		}
		if r == '/' {
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func normalizeBuildVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "v0.0.0"
	}
	return value
}

func buildVersionTag(value string) string {
	return imageTagToken(firstNonEmpty(normalizeBuildVersion(value), "v0.0.0"))
}

func normalizeSourceType(value SourceType) SourceType {
	switch SourceType(strings.ToLower(strings.TrimSpace(string(value)))) {
	case SourceTypeSVN:
		return SourceTypeSVN
	default:
		return SourceTypeGit
	}
}

func firstNonEmptySourceType(values ...SourceType) SourceType {
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			return value
		}
	}
	return SourceTypeGit
}

func defaultSourceRef(sourceType SourceType, sourceURL string) string {
	if normalizeSourceType(sourceType) == SourceTypeSVN {
		return svnTagRefFromURL(sourceURL)
	}
	return "main"
}

func normalizeSVNCheckoutPaths(sourceType SourceType, paths []SVNCheckoutPath) ([]SVNCheckoutPath, error) {
	if normalizeSourceType(sourceType) != SourceTypeSVN {
		return nil, nil
	}
	if len(paths) == 0 {
		return []SVNCheckoutPath{{Local: ".", Path: "", Depth: "infinity"}}, nil
	}
	out := make([]SVNCheckoutPath, 0, len(paths))
	seenLocal := map[string]struct{}{}
	for _, item := range paths {
		local, err := normalizeSVNRelativePath(item.Local, true)
		if err != nil {
			return nil, shared.NewError(shared.CodeInvalidArgument, "svn checkout local path is invalid")
		}
		relativePath, err := normalizeSVNRelativePath(item.Path, false)
		if err != nil {
			return nil, shared.NewError(shared.CodeInvalidArgument, "svn checkout path is invalid")
		}
		depth := strings.ToLower(strings.TrimSpace(item.Depth))
		if depth == "" {
			depth = "infinity"
		}
		if !allowedSVNCheckoutDepth(depth) {
			return nil, shared.NewError(shared.CodeInvalidArgument, "svn checkout depth is invalid")
		}
		if _, exists := seenLocal[local]; exists {
			return nil, shared.NewError(shared.CodeConflict, "svn checkout local path already exists")
		}
		seenLocal[local] = struct{}{}
		out = append(out, SVNCheckoutPath{Local: local, Path: relativePath, Depth: depth})
	}
	if len(out) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "svn checkout paths are required")
	}
	return out, nil
}

func normalizeSVNRelativePath(value string, allowRoot bool) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || value == "." {
		if allowRoot {
			return ".", nil
		}
		return "", nil
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\x00") {
		return "", fmt.Errorf("absolute path")
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		if allowRoot {
			return ".", nil
		}
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", fmt.Errorf("path traversal")
	}
	return cleaned, nil
}

func allowedSVNCheckoutDepth(value string) bool {
	switch value {
	case "empty", "files", "immediates", "infinity":
		return true
	default:
		return false
	}
}

func buildTemplateSVNCheckoutPaths(paths []SVNCheckoutPath) []buildTemplateSVNCheckoutPathView {
	if len(paths) == 0 {
		paths = []SVNCheckoutPath{{Local: ".", Path: "", Depth: "infinity"}}
	}
	out := make([]buildTemplateSVNCheckoutPathView, 0, len(paths))
	for _, item := range paths {
		depth := strings.TrimSpace(item.Depth)
		if depth == "" {
			depth = "infinity"
		}
		out = append(out, buildTemplateSVNCheckoutPathView{
			Local: shellSingleQuoted(item.Local),
			Path:  shellSingleQuoted(item.Path),
			Depth: shellSingleQuoted(depth),
		})
	}
	return out
}

func svnTagRefFromURL(sourceURL string) string {
	value := strings.Trim(strings.TrimSpace(sourceURL), "/")
	if value == "" {
		return "svn"
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		switch part {
		case "trunk":
			return "trunk"
		case "branches":
			if i+1 < len(parts) && strings.TrimSpace(parts[i+1]) != "" {
				return parts[i+1]
			}
		case "tags":
			if i+1 < len(parts) && strings.TrimSpace(parts[i+1]) != "" {
				return "tag-" + parts[i+1]
			}
		}
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.TrimSpace(parts[i]) != "" {
			return parts[i]
		}
	}
	return "svn"
}

func validateBuildVersion(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	core := value
	if strings.HasPrefix(core, "v") || strings.HasPrefix(core, "V") {
		core = core[1:]
	}
	if strings.Contains(core, "+") {
		return shared.NewError(shared.CodeInvalidArgument, "build version must be semver without build metadata")
	}
	parts := strings.SplitN(core, "-", 2)
	nums := strings.Split(parts[0], ".")
	if len(nums) != 3 {
		return shared.NewError(shared.CodeInvalidArgument, "build version must be semver")
	}
	for _, num := range nums {
		if num == "" {
			return shared.NewError(shared.CodeInvalidArgument, "build version must be semver")
		}
		for _, r := range num {
			if r < '0' || r > '9' {
				return shared.NewError(shared.CodeInvalidArgument, "build version must be semver")
			}
		}
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return shared.NewError(shared.CodeInvalidArgument, "build version must be semver")
		}
		for _, r := range parts[1] {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
				continue
			}
			return shared.NewError(shared.CodeInvalidArgument, "build version must be semver")
		}
	}
	return nil
}

func primaryBuildSourceKey(sources []ApplicationSourceRef, runSources []BuildRunSource) string {
	if source := primaryRunSource(runSources); strings.TrimSpace(source.SourceKey) != "" {
		return source.SourceKey
	}
	for _, source := range sources {
		if source.IsPrimary && strings.TrimSpace(source.Key) != "" {
			return source.Key
		}
	}
	if len(sources) > 0 {
		return sources[0].Key
	}
	return "main"
}

func groovyTripleSingleQuotedJSON(value any) string {
	encoded, _ := json.MarshalIndent(value, "", "  ")
	return strings.ReplaceAll(string(encoded), "'''", "'''\"'\"'''")
}

func groovyStringMapLiteral(values map[string]string) string {
	if len(values) == 0 {
		return "[:]"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("'%s': '%s'", groovyMapString(key), groovyMapString(values[key])))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func groovyMapString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "'", "\\'")
	value = strings.ReplaceAll(value, "\r", "\\r")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return value
}

func (s *Service) pipelineJobName(app ApplicationRef, pipelineName string) string {
	base := fmt.Sprintf("paas/%s/%s/%s", firstNonEmpty(app.TenantName, app.TenantID.String()), firstNonEmpty(app.ProjectName, app.ProjectID.String()), app.Name)
	if strings.TrimSpace(pipelineName) == "" {
		return base
	}
	return base + "/" + strings.TrimSpace(pipelineName)
}

func (s *Service) createBuildArtifacts(ctx context.Context, run BuildRun, input BuildCallbackInput) ([]BuildArtifact, error) {
	runSources, err := s.repo.ListRunSources(ctx, run.ID)
	if err != nil {
		return nil, err
	}
	sourceKeys := map[string]BuildRunSource{}
	for _, source := range runSources {
		sourceKeys[source.SourceKey] = source
	}
	inputs := input.Artifacts
	if len(inputs) == 0 && strings.TrimSpace(input.ImageURI) != "" {
		inputs = []BuildCallbackArtifactInput{{SourceKey: firstRunSourceKey(runSources), ContainerName: firstBoundContainerName(ctx, s, run.ApplicationID, run.PipelineID), Type: BuildArtifactImage, Name: "主镜像", URI: input.ImageURI, Digest: input.ImageDigest, IsPrimary: true}}
	}
	if len(inputs) == 0 {
		inputs, err = s.synthesizedArtifactInputs(ctx, run, runSources)
		if err != nil {
			return nil, err
		}
	}
	if len(inputs) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "artifacts is required for succeeded build")
	}
	primaryCount := 0
	artifacts := make([]BuildArtifact, 0, len(inputs))
	for i, item := range inputs {
		sourceKey := strings.TrimSpace(item.SourceKey)
		if sourceKey == "" && len(runSources) == 1 {
			sourceKey = runSources[0].SourceKey
		}
		source, ok := sourceKeys[sourceKey]
		if !ok {
			return nil, shared.NewError(shared.CodeInvalidArgument, "artifact source_key is not part of build run")
		}
		artifactType := item.Type
		if artifactType == "" {
			artifactType = BuildArtifactImage
		}
		switch artifactType {
		case BuildArtifactImage, BuildArtifactSBOM, BuildArtifactReport, BuildArtifactArchive:
		default:
			return nil, shared.NewError(shared.CodeInvalidArgument, "artifact type is not supported")
		}
		if strings.TrimSpace(item.URI) == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "artifact uri is required")
		}
		isPrimary := item.IsPrimary || (i == 0 && primaryCount == 0)
		if isPrimary {
			primaryCount++
			if primaryCount > 1 {
				return nil, shared.NewError(shared.CodeInvalidArgument, "only one artifact can be primary")
			}
		}
		id, err := s.ids.NewID("build_artifact")
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = source.SourceKey
		}
		containerName := normalizeContainerName(item.ContainerName)
		metadata := map[string]string{"commit_sha": source.CommitSHA, "source_type": string(source.SourceType), "source_ref": source.SourceRef, "source_url": source.SourceURL, "container_name": containerName}
		for k, v := range item.Metadata {
			metadata[k] = v
		}
		artifact := BuildArtifact{ID: id, TenantID: run.TenantID, ProjectID: run.ProjectID, BuildRunID: run.ID, ApplicationID: run.ApplicationID, WorkloadID: run.WorkloadID, ContainerName: containerName, SourceKey: source.SourceKey, Type: artifactType, Name: name, URI: strings.TrimSpace(item.URI), Digest: strings.TrimSpace(item.Digest), IsPrimary: isPrimary, SelectorLabels: normalizeSelectorLabels(item.SelectorLabels), Metadata: metadata, CreatedAt: s.clock.Now()}
		if err := s.repo.CreateArtifact(ctx, artifact); err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func firstBoundContainerName(ctx context.Context, s *Service, applicationID shared.ID, pipelineID shared.ID) string {
	if s == nil || s.workloads == nil || pipelineID.IsZero() {
		return "app"
	}
	workloads, err := s.workloads.ListEnabledWorkloadsByPipeline(ctx, applicationID, pipelineID)
	if err != nil {
		return "app"
	}
	for _, workload := range workloads {
		if name := normalizeContainerName(workload.ContainerName); name != "" {
			return name
		}
	}
	return "app"
}

func firstRunSourceKey(sources []BuildRunSource) string {
	if len(sources) == 0 {
		return ""
	}
	return sources[0].SourceKey
}

func (s *Service) synthesizedArtifactInputs(ctx context.Context, run BuildRun, runSources []BuildRunSource) ([]BuildCallbackArtifactInput, error) {
	app, err := s.requireApplication(ctx, run.ApplicationID)
	if err != nil {
		return nil, err
	}
	pipeline, err := s.repo.GetPipeline(ctx, run.PipelineID)
	if err != nil {
		return nil, err
	}
	imageRepo := strings.TrimRight(strings.TrimSpace(s.imageRepository), "/")
	if imageRepo == "" {
		imageRepo = "registry.local/paas"
	}
	runtimes := nonEmptyRuntimeEnvironments(pipeline.RuntimeEnvironments)
	if len(runtimes) > 0 {
		source := primaryRunSource(runSources)
		if source.SourceKey == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "artifact source_key is not part of build run")
		}
		baseTag := buildImageBaseTag(runSources, run)
		out := make([]BuildCallbackArtifactInput, 0, len(runtimes))
		targetIndex := 0
		for _, runtime := range runtimes {
			images := runtimeEnvironmentImagesOrLegacy(runtime.Images, runtime.ID, runtime.Name, runtime.RuntimeBaseImage, runtime.ArtifactDeployPath, runtime.DockerfilePath, runtime.SelectorLabels)
			for _, image := range images {
				name := firstNonEmpty(image.DisplayName, image.Name, runtime.Name)
				key := shellToken(firstNonEmpty(runtime.Name+"-"+image.Name, image.Name, runtime.Name))
				if key == "" {
					key = fmt.Sprintf("runtime%d", targetIndex+1)
				}
				imageRepository := buildTargetImageRepository(imageRepo, app.Name, image.Name)
				out = append(out, BuildCallbackArtifactInput{
					SourceKey:      source.SourceKey,
					ContainerName:  firstBoundContainerName(ctx, s, run.ApplicationID, run.PipelineID),
					Type:           BuildArtifactImage,
					Name:           name,
					URI:            imageRepository + ":" + baseTag,
					IsPrimary:      targetIndex == 0,
					SelectorLabels: image.SelectorLabels,
					Metadata: map[string]string{
						"runtime_environment_id":         runtime.ID.String(),
						"runtime_environment_name":       runtime.Name,
						"runtime_environment_image_id":   image.ID.String(),
						"runtime_environment_image_name": image.Name,
						"runtime_base_image":             image.RuntimeBaseImage,
						"image_repository":               imageRepository,
						"architectures":                  strings.Join(normalizeImageArchitectures(image.Architectures), ","),
					},
				})
				targetIndex++
			}
		}
		return out, nil
	}
	out := make([]BuildCallbackArtifactInput, 0, len(runSources))
	baseTag := buildImageBaseTag(runSources, run)
	for i, source := range runSources {
		imageRepository := buildTargetImageRepository(imageRepo, app.Name, source.SourceKey)
		out = append(out, BuildCallbackArtifactInput{SourceKey: source.SourceKey, ContainerName: firstBoundContainerName(ctx, s, run.ApplicationID, run.PipelineID), Type: BuildArtifactImage, Name: source.SourceKey, URI: fmt.Sprintf("%s:%s", imageRepository, baseTag), IsPrimary: i == 0})
	}
	return out, nil
}

func primaryRunSource(sources []BuildRunSource) BuildRunSource {
	for _, source := range sources {
		if source.IsPrimary {
			return source
		}
	}
	if len(sources) == 0 {
		return BuildRunSource{}
	}
	return sources[0]
}

func runtimeArtifactName(runtime RuntimeEnvironmentRef) string {
	if strings.TrimSpace(runtime.Name) != "" {
		return strings.TrimSpace(runtime.Name)
	}
	if !runtime.ID.IsZero() {
		return runtime.ID.String()
	}
	return "runtime"
}

func nonEmptyRuntimeEnvironments(runtimes []RuntimeEnvironmentRef) []RuntimeEnvironmentRef {
	out := make([]RuntimeEnvironmentRef, 0, len(runtimes))
	for _, runtime := range runtimes {
		if runtime.ID.IsZero() && strings.TrimSpace(runtime.Name) == "" && strings.TrimSpace(runtime.RuntimeBaseImage) == "" {
			continue
		}
		out = append(out, runtime)
	}
	return out
}

func buildRuntimePayload(pipelineRuntimes []RuntimeEnvironmentRef, sources []ApplicationSourceRef) []map[string]string {
	if pipelineRuntimes := nonEmptyRuntimeEnvironments(pipelineRuntimes); len(pipelineRuntimes) > 0 {
		runtimes := make([]map[string]string, 0, len(pipelineRuntimes))
		for _, runtime := range pipelineRuntimes {
			runtimes = append(runtimes, map[string]string{
				"id":                   runtime.ID.String(),
				"name":                 runtime.Name,
				"runtime_base_image":   runtime.RuntimeBaseImage,
				"artifact_deploy_path": runtime.ArtifactDeployPath,
				"dockerfile_path":      runtime.DockerfilePath,
			})
		}
		return runtimes
	}
	if len(sources) == 0 {
		return []map[string]string{}
	}
	spec := sources[0].BuildSpec
	return []map[string]string{{
		"runtime_base_image":   spec.RuntimeBaseImage,
		"artifact_deploy_path": spec.ArtifactDeployPath,
	}}
}

func groovySingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}

func shellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "'\"'\"'")
}

func normalizeShellBlock(script string) string {
	return strings.TrimSpace(script)
}

func (s *Service) buildCallbackURL(run BuildRun) string {
	base := strings.TrimRight(strings.TrimSpace(s.callbackURL), "/")
	if base == "" {
		return ""
	}
	return base + "/api/builds/" + run.ID.String() + "/callback"
}

func buildSourceOverrides(inputs []TriggerBuildSourceInput) map[string]TriggerBuildSourceInput {
	out := map[string]TriggerBuildSourceInput{}
	for _, input := range inputs {
		key := strings.TrimSpace(input.Key)
		if key == "" {
			continue
		}
		input.Key = key
		out[key] = input
	}
	return out
}

func (input TriggerBuildInput) normalizedSourceOverrides(primaryKey string) []TriggerBuildSourceInput {
	if len(input.Sources) > 0 {
		return input.Sources
	}
	if strings.TrimSpace(input.SourceRef) == "" && strings.TrimSpace(input.CommitSHA) == "" {
		return nil
	}
	return []TriggerBuildSourceInput{{Key: primaryKey, SourceRef: input.SourceRef, CommitSHA: input.CommitSHA}}
}

func (s *Service) newBuildRunSources(ctx context.Context, runID shared.ID, app ApplicationRef, sources []ApplicationSourceRef, overrides map[string]TriggerBuildSourceInput, now time.Time) ([]BuildRunSource, error) {
	items := make([]BuildRunSource, 0, len(sources))
	seen := map[string]struct{}{}
	for _, source := range sources {
		seen[source.Key] = struct{}{}
	}
	for key := range overrides {
		if _, ok := seen[key]; !ok {
			return nil, shared.NewError(shared.CodeInvalidArgument, "build source override key is not part of application")
		}
	}
	for _, source := range sources {
		override := overrides[source.Key]
		sourceRef := strings.TrimSpace(override.SourceRef)
		if sourceRef == "" {
			sourceRef = strings.TrimSpace(source.SourceRef)
		}
		if sourceRef == "" {
			sourceRef = defaultSourceRef(source.SourceType, source.SourceURL)
		}
		id, err := s.ids.NewID("build_run_source")
		if err != nil {
			return nil, err
		}
		items = append(items, BuildRunSource{
			ID:               id,
			TenantID:         app.TenantID,
			ProjectID:        app.ProjectID,
			BuildRunID:       runID,
			ApplicationID:    app.ID,
			SourceKey:        source.Key,
			SourceType:       normalizeSourceType(source.SourceType),
			SourceURL:        source.SourceURL,
			SourceRef:        sourceRef,
			SVNCheckoutPaths: append([]SVNCheckoutPath(nil), source.SVNCheckoutPaths...),
			CommitSHA:        strings.TrimSpace(override.CommitSHA),
			SourcePath:       source.BuildSpec.SourcePath,
			IsPrimary:        source.IsPrimary,
			CreatedAt:        now,
		})
	}
	return items, nil
}

func (s *Service) loadBuildContext(ctx context.Context, applicationID shared.ID) (ApplicationRef, []ApplicationSourceRef, error) {
	app, err := s.requireApplication(ctx, applicationID)
	if err != nil {
		return ApplicationRef{}, nil, err
	}
	if s.apps == nil {
		return ApplicationRef{}, nil, shared.NewError(shared.CodeFailedPrecondition, "application query port is required")
	}
	sources, err := s.apps.ListApplicationSources(ctx, applicationID)
	if err != nil {
		if shared.CodeOf(err) != shared.CodeNotFound {
			return ApplicationRef{}, nil, err
		}
		source, sourceErr := s.apps.GetApplicationSource(ctx, applicationID)
		if sourceErr != nil {
			return ApplicationRef{}, nil, sourceErr
		}
		sources = []ApplicationSourceRef{source}
	}
	if len(sources) == 0 {
		return ApplicationRef{}, nil, shared.NewError(shared.CodeNotFound, "application source not found")
	}
	for i := range sources {
		if sources[i].Key == "" {
			sources[i].Key = "main"
		}
		if sources[i].SourcePath == "" {
			sources[i].SourcePath = sources[i].BuildSpec.SourcePath
		}
		if !sources[i].IsPrimary && i == 0 {
			sources[i].IsPrimary = true
		}
	}
	return app, sources, nil
}

func (s *Service) loadPipelineBuildContext(ctx context.Context, pipelineID shared.ID) (ApplicationRef, BuildPipeline, []ApplicationSourceRef, error) {
	if pipelineID.IsZero() {
		return ApplicationRef{}, BuildPipeline{}, nil, shared.NewError(shared.CodeInvalidArgument, "pipeline_id is required")
	}
	pipeline, err := s.repo.GetPipeline(ctx, pipelineID)
	if err != nil {
		return ApplicationRef{}, BuildPipeline{}, nil, err
	}
	if pipeline.Status != BuildPipelineStatusActive {
		return ApplicationRef{}, BuildPipeline{}, nil, shared.NewError(shared.CodeFailedPrecondition, "build pipeline is disabled")
	}
	app, err := s.requireApplication(ctx, pipeline.ApplicationID)
	if err != nil {
		return ApplicationRef{}, BuildPipeline{}, nil, err
	}
	pipelineSources, err := s.repo.ListPipelineSources(ctx, pipeline.ID)
	if err != nil {
		return ApplicationRef{}, BuildPipeline{}, nil, err
	}
	if len(pipelineSources) == 0 {
		return ApplicationRef{}, BuildPipeline{}, nil, shared.NewError(shared.CodeNotFound, "build pipeline source not found")
	}
	sources := make([]ApplicationSourceRef, 0, len(pipelineSources))
	for i, source := range pipelineSources {
		key := strings.TrimSpace(source.Key)
		if key == "" {
			key = "main"
		}
		spec := source.BuildSpec
		if spec.SourcePath == "" {
			spec.SourcePath = source.SourcePath
		}
		if !source.IsPrimary && i == 0 {
			source.IsPrimary = true
		}
		sources = append(sources, ApplicationSourceRef{
			ApplicationID:      source.ApplicationID,
			Key:                key,
			DisplayName:        source.DisplayName,
			BuildEnvironmentID: source.BuildEnvironmentID,
			SourceType:         source.SourceType,
			SourceURL:          source.SourceURL,
			SourceRef:          source.SourceRef,
			SVNRevision:        source.SVNRevision,
			SVNCheckoutPaths:   append([]SVNCheckoutPath(nil), source.SVNCheckoutPaths...),
			SourcePath:         source.SourcePath,
			BuildSpec:          spec,
			IsPrimary:          source.IsPrimary,
		})
	}
	return app, pipeline, sources, nil
}

func (s *Service) preparePipelineSources(ctx context.Context, app ApplicationRef, pipelineID shared.ID, inputs []BuildPipelineSourceInput, runtimes []RuntimeEnvironmentRef) ([]BuildPipelineSource, error) {
	if len(inputs) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "pipeline sources are required")
	}
	if len(runtimes) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "runtime_environment_ids is required")
	}
	sources := make([]BuildPipelineSource, 0, len(inputs))
	seen := map[string]struct{}{}
	primaryIndex := -1
	for i, input := range inputs {
		key := normalizePipelineName(input.Key)
		if key == "" && len(inputs) == 1 {
			key = "main"
		}
		if err := validatePipelineName(key); err != nil {
			return nil, err
		}
		if _, ok := seen[key]; ok {
			return nil, shared.NewError(shared.CodeConflict, "pipeline source key already exists")
		}
		seen[key] = struct{}{}
		sourceType := normalizeSourceType(input.SourceType)
		sourceURL := strings.TrimSpace(input.SourceURL)
		if sourceURL == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "source_url is required")
		}
		sourceRef := strings.TrimSpace(input.SourceRef)
		spec := input.BuildSpec
		if strings.TrimSpace(spec.SourcePath) == "" {
			spec.SourcePath = input.SourcePath
		}
		if strings.TrimSpace(input.DefaultRef) != "" {
			spec.DefaultRef = strings.TrimSpace(input.DefaultRef)
		}
		if sourceRef == "" {
			sourceRef = strings.TrimSpace(spec.DefaultRef)
		}
		if sourceRef == "" {
			sourceRef = defaultSourceRef(sourceType, sourceURL)
		}
		spec.DefaultRef = sourceRef
		applyPipelineRuntime(runtimes[0], &spec)
		if err := validateBuildSpec(spec); err != nil {
			return nil, err
		}
		svnRevision := strings.TrimSpace(input.SVNRevision)
		svnCheckoutPaths, err := normalizeSVNCheckoutPaths(sourceType, input.SVNCheckoutPaths)
		if err != nil {
			return nil, err
		}
		sourceID, err := s.ids.NewID("build_pipeline_source")
		if err != nil {
			return nil, err
		}
		sourcePath := spec.SourcePath
		isPrimary := input.IsPrimary
		if isPrimary {
			if primaryIndex >= 0 {
				return nil, shared.NewError(shared.CodeInvalidArgument, "only one source can be primary")
			}
			primaryIndex = i
		}
		sources = append(sources, BuildPipelineSource{
			ID:                 sourceID,
			TenantID:           app.TenantID,
			ProjectID:          app.ProjectID,
			ApplicationID:      app.ID,
			PipelineID:         pipelineID,
			Key:                key,
			DisplayName:        normalizeDisplayName(input.DisplayName, key),
			SourceType:         sourceType,
			SourceURL:          sourceURL,
			SourceRef:          sourceRef,
			SVNRevision:        svnRevision,
			SVNCheckoutPaths:   svnCheckoutPaths,
			BuildEnvironmentID: input.BuildEnvironmentID,
			SourcePath:         sourcePath,
			BuildSpec:          spec,
			IsPrimary:          isPrimary,
		})
	}
	if primaryIndex < 0 {
		sources[0].IsPrimary = true
	} else if primaryIndex != 0 {
		primary := sources[primaryIndex]
		copy(sources[1:primaryIndex+1], sources[0:primaryIndex])
		sources[0] = primary
	}
	return sources, nil
}

func (s *Service) requireEnabledRuntimeEnvironments(ctx context.Context, ids []shared.ID) ([]RuntimeEnvironmentRef, error) {
	if len(ids) != 1 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "请选择一个运行时环境")
	}
	out := make([]RuntimeEnvironmentRef, 0, len(ids))
	seen := map[shared.ID]struct{}{}
	for _, id := range ids {
		if id.IsZero() {
			return nil, shared.NewError(shared.CodeInvalidArgument, "runtime_environment_id is required")
		}
		environment, err := s.resolveRuntimeEnvironment(ctx, id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[environment.ID]; ok {
			return nil, shared.NewError(shared.CodeConflict, "runtime environment already selected")
		}
		seen[environment.ID] = struct{}{}
		if environment.Status != RuntimeEnvironmentEnabled {
			return nil, shared.NewError(shared.CodeFailedPrecondition, "runtime environment is disabled")
		}
		environment.Images = normalizeRuntimeEnvironmentImages(environment.Images)
		if err := validateRuntimeEnvironment(environment); err != nil {
			return nil, err
		}
		out = append(out, RuntimeEnvironmentRef{
			ID:                 environment.ID,
			Name:               environment.Name,
			RuntimeBaseImage:   environment.RuntimeBaseImage,
			ArtifactDeployPath: environment.ArtifactDeployPath,
			DockerfilePath:     environment.DockerfilePath,
			SelectorLabels:     normalizeSelectorLabels(environment.SelectorLabels),
			Images:             environment.Images,
		})
	}
	return out, nil
}

func (s *Service) resolveRuntimeEnvironment(ctx context.Context, id shared.ID) (RuntimeEnvironment, error) {
	environment, err := s.repo.GetRuntimeEnvironment(ctx, id)
	if err == nil {
		return environment, nil
	}
	if shared.CodeOf(err) != shared.CodeNotFound {
		return RuntimeEnvironment{}, err
	}
	page, listErr := s.repo.ListRuntimeEnvironments(ctx, false, shared.PageRequest{Page: 1, PageSize: shared.MaxPageSize})
	if listErr != nil {
		return RuntimeEnvironment{}, listErr
	}
	name := strings.TrimSpace(id.String())
	for _, environment := range page.Items {
		if environment.Name == name {
			return environment, nil
		}
	}
	return RuntimeEnvironment{}, err
}

func applyPipelineRuntime(runtime RuntimeEnvironmentRef, spec *BuildSpec) {
	spec.RuntimeBaseImage = runtime.RuntimeBaseImage
	spec.ArtifactDeployPath = runtime.ArtifactDeployPath
}

func (s *Service) requireApplication(ctx context.Context, applicationID shared.ID) (ApplicationRef, error) {
	if applicationID.IsZero() {
		return ApplicationRef{}, shared.NewError(shared.CodeInvalidArgument, "application_id is required")
	}
	if s.apps == nil {
		return ApplicationRef{}, shared.NewError(shared.CodeFailedPrecondition, "application query port is required")
	}
	return s.apps.GetApplication(ctx, applicationID)
}

func (s *Service) resolveJenkinsJobTemplate(ctx context.Context, templateID shared.ID) (JenkinsJobTemplate, error) {
	var (
		template JenkinsJobTemplate
		err      error
	)
	if templateID.IsZero() {
		template, err = s.repo.FindDefaultJenkinsJobTemplate(ctx)
	} else {
		template, err = s.repo.GetJenkinsJobTemplate(ctx, templateID)
	}
	if err != nil {
		if templateID.IsZero() && shared.CodeOf(err) == shared.CodeNotFound {
			return s.builtinJenkinsJobTemplate(), nil
		}
		return JenkinsJobTemplate{}, err
	}
	if template.Status != JenkinsJobTemplateEnabled {
		return JenkinsJobTemplate{}, shared.NewError(shared.CodeFailedPrecondition, "jenkins template is disabled")
	}
	if err := validateJenkinsfile(template.XMLContent); err != nil {
		return JenkinsJobTemplate{}, err
	}
	return template, nil
}

func (s *Service) builtinJenkinsJobTemplate() JenkinsJobTemplate {
	now := s.clock.Now()
	return JenkinsJobTemplate{ID: shared.ID(s.templateID), Name: s.templateID, DisplayName: s.templateID, Version: 1, XMLContent: defaultJenkinsfile, Status: JenkinsJobTemplateEnabled, IsDefault: true, CreatedAt: now, UpdatedAt: now}
}

func (s *Service) check(ctx context.Context, actor identityaccess.Subject, app ApplicationRef, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID}, action)
}

func (s *Service) checkPlatformAdmin(ctx context.Context, actor identityaccess.Subject, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopePlatform}, action)
}

func (s *Service) runnerOrError() BuildRunnerPort {
	if s.runner == nil {
		return failingRunner{}
	}
	return s.runner
}

func (s *Service) publish(ctx context.Context, eventType string, occurredAt time.Time, payload any) error {
	id, err := s.ids.NewID("evt")
	if err != nil {
		return err
	}
	event, err := shared.NewDomainEvent(id, eventType, occurredAt, payload)
	if err != nil {
		return err
	}
	return s.events.Publish(ctx, event)
}

func buildSpecAuditDetails(spec BuildSpec) map[string]string {
	return map[string]string{
		"build_command":         spec.BuildCommand,
		"artifact_copy_command": spec.ArtifactCopyCommand,
		"runtime_base_image":    spec.RuntimeBaseImage,
		"artifact_deploy_path":  spec.ArtifactDeployPath,
	}
}

func normalizeTemplateName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizePipelineName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' && !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func validatePipelineName(value string) error {
	if value == "" || len(value) > 63 {
		return shared.NewError(shared.CodeInvalidArgument, "pipeline name is required")
	}
	if value[0] < 'a' || value[0] > 'z' {
		return shared.NewError(shared.CodeInvalidArgument, "pipeline name must start with a lowercase letter")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return shared.NewError(shared.CodeInvalidArgument, "pipeline name must contain lowercase letters, numbers or hyphens")
	}
	return nil
}

func normalizeDisplayName(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func shellToken(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '/' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

func shellEnvName(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			continue
		}
		if r == '-' || r == '.' || r == '/' {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "TARGET"
	}
	return b.String()
}

func groovyStageName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "'", "")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if value == "" {
		return "source"
	}
	return value
}

func (s *Service) newBuildEnvironment(actorID shared.ID, name, description, buildImage string, isDefault bool) (BuildEnvironment, error) {
	id, err := s.ids.NewID("build_env")
	if err != nil {
		return BuildEnvironment{}, err
	}
	now := s.clock.Now()
	environment := BuildEnvironment{
		ID:          id,
		Name:        normalizeTemplateName(name),
		Description: strings.TrimSpace(description),
		BuildImage:  strings.TrimSpace(buildImage),
		Status:      BuildEnvironmentEnabled,
		IsDefault:   isDefault,
		CreatedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := validateBuildEnvironment(environment); err != nil {
		return BuildEnvironment{}, err
	}
	return environment, nil
}

func validateBuildEnvironment(environment BuildEnvironment) error {
	if strings.TrimSpace(environment.Name) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "build environment name is required")
	}
	if strings.TrimSpace(environment.BuildImage) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "build_image is required")
	}
	return nil
}

func defaultBuildImage() string {
	return "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/gradle:7-jdk11"
}

func (s *Service) newRuntimeEnvironment(actorID shared.ID, name, description, runtimeBaseImage, artifactDeployPath, dockerfilePath string, selectorLabels map[string]string, images []RuntimeEnvironmentImage) (RuntimeEnvironment, error) {
	id, err := s.ids.NewID("runtime_env")
	if err != nil {
		return RuntimeEnvironment{}, err
	}
	now := s.clock.Now()
	environment := RuntimeEnvironment{
		ID:          id,
		Name:        normalizeTemplateName(name),
		Description: strings.TrimSpace(description),
		Images:      normalizeRuntimeEnvironmentImages(images),
		Status:      RuntimeEnvironmentEnabled,
		CreatedBy:   actorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if len(environment.Images) == 0 {
		environment.Images = runtimeEnvironmentImagesOrLegacy(nil, environment.ID, environment.Name, runtimeBaseImage, artifactDeployPath, dockerfilePath, selectorLabels)
	}
	applyRuntimeEnvironmentFirstImage(&environment)
	if err := validateRuntimeEnvironment(environment); err != nil {
		return RuntimeEnvironment{}, err
	}
	return environment, nil
}

func validateRuntimeEnvironment(environment RuntimeEnvironment) error {
	if strings.TrimSpace(environment.Name) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "runtime environment name is required")
	}
	if len(environment.Images) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "请至少配置一个运行时镜像")
	}
	seenNames := map[string]struct{}{}
	seenLabels := map[string]struct{}{}
	for _, image := range environment.Images {
		if strings.TrimSpace(image.Name) == "" {
			return shared.NewError(shared.CodeInvalidArgument, "运行时镜像名称不能为空")
		}
		if _, ok := seenNames[image.Name]; ok {
			return shared.NewError(shared.CodeConflict, "运行时镜像名称重复")
		}
		seenNames[image.Name] = struct{}{}
		if strings.TrimSpace(image.RuntimeBaseImage) == "" {
			return shared.NewError(shared.CodeInvalidArgument, "runtime_base_image is required")
		}
		if len(image.SelectorLabels) == 0 {
			return shared.NewError(shared.CodeInvalidArgument, "运行时镜像匹配标签不能为空")
		}
		if len(image.Architectures) == 0 {
			return shared.NewError(shared.CodeInvalidArgument, "运行时镜像至少选择一个 CPU 架构")
		}
		labelKey := selectorLabelKey(image.SelectorLabels)
		if _, ok := seenLabels[labelKey]; ok {
			return shared.NewError(shared.CodeConflict, "运行时镜像匹配标签重复")
		}
		seenLabels[labelKey] = struct{}{}
		deployPath := strings.TrimSpace(image.ArtifactDeployPath)
		if deployPath != "" && (!strings.HasPrefix(deployPath, "/") || strings.Contains(deployPath, "..")) {
			return shared.NewError(shared.CodeInvalidArgument, "artifact_deploy_path must be absolute and stay under runtime root")
		}
		dockerfilePath := strings.TrimSpace(image.DockerfilePath)
		if dockerfilePath != "" {
			if _, err := normalizeRelativePath(dockerfilePath); err != nil {
				return shared.NewError(shared.CodeInvalidArgument, "dockerfile_path must be relative and stay under Dockerfile repository root")
			}
		}
	}
	return nil
}

func normalizeRuntimeEnvironmentImages(images []RuntimeEnvironmentImage) []RuntimeEnvironmentImage {
	if len(images) == 0 {
		return nil
	}
	out := make([]RuntimeEnvironmentImage, 0, len(images))
	for _, image := range images {
		name := normalizeTemplateName(image.Name)
		if name == "" {
			name = normalizeTemplateName(image.DisplayName)
		}
		if name == "" && !image.ID.IsZero() {
			name = image.ID.String()
		}
		status := strings.TrimSpace(image.Status)
		if status == "" {
			status = string(RuntimeEnvironmentEnabled)
		}
		out = append(out, RuntimeEnvironmentImage{
			ID:                 image.ID,
			Name:               name,
			DisplayName:        normalizeDisplayName(image.DisplayName, name),
			Architectures:      normalizeImageArchitectures(image.Architectures),
			RuntimeBaseImage:   strings.TrimSpace(image.RuntimeBaseImage),
			ArtifactDeployPath: strings.TrimSpace(image.ArtifactDeployPath),
			DockerfilePath:     normalizeDockerfilePath(image.DockerfilePath),
			SelectorLabels:     normalizeSelectorLabels(image.SelectorLabels),
			Status:             status,
		})
	}
	return out
}

func applyRuntimeEnvironmentFirstImage(environment *RuntimeEnvironment) {
	environment.Images = normalizeRuntimeEnvironmentImages(environment.Images)
	if len(environment.Images) == 0 {
		return
	}
	first := environment.Images[0]
	environment.RuntimeBaseImage = first.RuntimeBaseImage
	environment.ArtifactDeployPath = first.ArtifactDeployPath
	environment.DockerfilePath = first.DockerfilePath
	environment.SelectorLabels = normalizeSelectorLabels(first.SelectorLabels)
}

func selectorLabelKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for key, value := range labels {
		parts = append(parts, strings.TrimSpace(key)+"="+strings.TrimSpace(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func normalizeSelectorLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range labels {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeDockerfilePath(value string) string {
	normalized, err := normalizeRelativePath(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return normalized
}

func validateBuildTemplateContent(content string) error {
	if err := validateJenkinsfile(content); err != nil {
		return err
	}
	tpl, err := template.New("build-template").Parse(content)
	if err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "build template syntax is invalid", err)
	}
	var b strings.Builder
	if err := tpl.Execute(&b, sampleBuildTemplateView()); err != nil {
		return shared.NewError(shared.CodeInvalidArgument, "build template render failed: "+err.Error())
	}
	return nil
}

func sampleBuildTemplateView() buildTemplateView {
	return buildTemplateView{
		AgentLabel: "docker",
		Runtime: map[string]string{
			"runtime_base_image":   "registry.example/runtime/java17:1.0",
			"artifact_deploy_path": "/app/",
			"application_name":     "sample-app",
		},
		DockerfileRepository: buildTemplateDockerfileRepositoryView{URL: "git@example.com:platform/dockerfiles.git", Ref: "main"},
		Sources: []buildTemplateSourceView{{
			Key:              "main",
			StageName:        "main",
			SourceType:       "git",
			RepoURL:          "git@example.com:sample/repo.git",
			SourceRef:        "main",
			SourceRefB64:     "bWFpbg==",
			SVNRevision:      "HEAD",
			SVNCredentials:   defaultSVNCredentialsID,
			SVNCheckoutPaths: []buildTemplateSVNCheckoutPathView{{Local: ".", Path: "", Depth: "infinity"}},
			CheckoutDir:      "source/main",
			WorkDir:          "source/main",
			BuildImage:       "maven:3.9.9-eclipse-temurin-17",
			BuildCommand:     "mvn clean package -DskipTests",
			CollectCommand:   "cp -ar target/app.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
		}},
	}
}

func (s *Service) ensureDefaultBuildEnvironments(ctx context.Context, actorID shared.ID) error {
	now := s.clock.Now()
	existingPage, err := s.repo.ListBuildEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	seedMissingDefaults := existingPage.Total == 0
	defaults := []BuildEnvironment{
		{ID: "build_env_gradle7_jdk11", Name: "gradle7-jdk11", BuildImage: "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/gradle:7-jdk11", Status: BuildEnvironmentEnabled, IsDefault: true, CreatedBy: actorID, CreatedAt: now, UpdatedAt: now},
		{ID: "build_env_node22", Name: "node22", BuildImage: "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/node:22.14.0-bookworm", Status: BuildEnvironmentEnabled, CreatedBy: actorID, CreatedAt: now, UpdatedAt: now},
	}
	for _, environment := range defaults {
		existing, err := s.repo.GetBuildEnvironment(ctx, environment.ID)
		if err == nil {
			if existing.Status == BuildEnvironmentDeleted {
				continue
			}
			changed := false
			if strings.TrimSpace(existing.BuildImage) == "" {
				existing.BuildImage = environment.BuildImage
				changed = true
			}
			if changed {
				existing.UpdatedAt = now
				if err := s.repo.UpdateBuildEnvironment(ctx, existing); err != nil {
					return err
				}
			}
			continue
		}
		if shared.CodeOf(err) != shared.CodeNotFound {
			return err
		}
		if !seedMissingDefaults {
			continue
		}
		if err := racyCreateBuildEnvironment(ctx, s.repo, environment); err != nil {
			return err
		}
	}
	return nil
}

func racyCreateBuildEnvironment(ctx context.Context, repo Repository, environment BuildEnvironment) error {
	if err := repo.CreateBuildEnvironment(ctx, environment); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		return err
	}
	return nil
}

func (s *Service) ensureDefaultRuntimeEnvironments(ctx context.Context, actorID shared.ID) error {
	existingPage, err := s.repo.ListRuntimeEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	seedMissingDefaults := existingPage.Total == 0
	now := s.clock.Now()
	defaults := []RuntimeEnvironment{
		runtimeEnvironmentDefault("runtime_env_springboot_jdk11", "springboot-jdk11", actorID, now, []RuntimeEnvironmentImage{
			runtimeEnvironmentImageDefault("runtime_image_springboot_jdk11_aliyun", "aliyun", "阿里云 Dragonwell", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/dragonwell:11-anolis", "java/jar/Dockerfile", map[string]string{"cloud": "aliyun"}),
			runtimeEnvironmentImageDefault("runtime_image_springboot_jdk11_aws", "aws", "AWS Corretto", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/amazoncorretto:11-al2023", "java/jar/Dockerfile", map[string]string{"cloud": "aws"}),
		}),
		runtimeEnvironmentDefault("runtime_env_tomcat_jdk11", "tomcat-jdk11", actorID, now, []RuntimeEnvironmentImage{
			runtimeEnvironmentImageDefault("runtime_image_tomcat_jdk11_aliyun", "aliyun", "阿里云 Tomcat", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/tomcat:8.5.87-dragonwell11-anolis", "java/tomcat/Dockerfile", map[string]string{"cloud": "aliyun"}),
			runtimeEnvironmentImageDefault("runtime_image_tomcat_jdk11_aws", "aws", "AWS Tomcat", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/tomcat:8.5.87-corretto11-al2023", "java/tomcat/Dockerfile", map[string]string{"cloud": "aws"}),
		}),
		runtimeEnvironmentDefault("runtime_env_nginx1221", "nginx1221", actorID, now, []RuntimeEnvironmentImage{
			runtimeEnvironmentImageDefault("runtime_image_nginx1221_aliyun", "aliyun", "阿里云 Nginx", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/nginx:1.22.1", "nginx/Dockerfile", map[string]string{"cloud": "aliyun"}),
			runtimeEnvironmentImageDefault("runtime_image_nginx1221_aws", "aws", "AWS Nginx", "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/nginx:1.22.1", "nginx/Dockerfile", map[string]string{"cloud": "aws"}),
		}),
	}
	for _, environment := range defaults {
		existing, err := s.repo.GetRuntimeEnvironment(ctx, environment.ID)
		if err == nil {
			if existing.Status == RuntimeEnvironmentDeleted {
				continue
			}
			if len(existing.Images) == 0 {
				existing.Images = environment.Images
				applyRuntimeEnvironmentFirstImage(&existing)
				existing.UpdatedAt = now
				if err := s.repo.UpdateRuntimeEnvironment(ctx, existing); err != nil {
					return err
				}
			}
			continue
		}
		if shared.CodeOf(err) != shared.CodeNotFound {
			return err
		}
		if !seedMissingDefaults {
			continue
		}
		if err := repoCreateRuntimeEnvironment(ctx, s.repo, environment); err != nil {
			return err
		}
	}
	return nil
}

func repoCreateRuntimeEnvironment(ctx context.Context, repo Repository, environment RuntimeEnvironment) error {
	if err := repo.CreateRuntimeEnvironment(ctx, environment); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		return err
	}
	return nil
}

func runtimeEnvironmentDefault(id, name string, actorID shared.ID, now time.Time, images []RuntimeEnvironmentImage) RuntimeEnvironment {
	environment := RuntimeEnvironment{ID: shared.ID(id), Name: name, Images: normalizeRuntimeEnvironmentImages(images), Status: RuntimeEnvironmentEnabled, CreatedBy: actorID, CreatedAt: now, UpdatedAt: now}
	applyRuntimeEnvironmentFirstImage(&environment)
	return environment
}

func runtimeEnvironmentImageDefault(id, name, displayName, baseImage, dockerfilePath string, labels map[string]string) RuntimeEnvironmentImage {
	return RuntimeEnvironmentImage{
		ID:               shared.ID(id),
		Name:             name,
		DisplayName:      displayName,
		Architectures:    defaultImageArchitectures(),
		RuntimeBaseImage: baseImage,
		DockerfilePath:   dockerfilePath,
		SelectorLabels:   labels,
		Status:           string(RuntimeEnvironmentEnabled),
	}
}

func normalizeSensitiveValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeDockerfileRepositoryConfig(config DockerfileRepositoryConfig) DockerfileRepositoryConfig {
	config.URL = strings.TrimSpace(config.URL)
	config.Ref = strings.TrimSpace(config.Ref)
	if config.Ref == "" && config.URL != "" {
		config.Ref = "main"
	}
	config.CredentialsID = strings.TrimSpace(config.CredentialsID)
	return config
}

func redactAssignment(text string, marker string) string {
	var out strings.Builder
	for {
		idx := strings.Index(text, marker)
		if idx < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:idx+len(marker)])
		out.WriteString("******")
		text = text[idx+len(marker):]
		end := len(text)
		for i, r := range text {
			if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
				end = i
				break
			}
		}
		text = text[end:]
	}
}

type failingRunner struct{}

func (failingRunner) EnsureJob(context.Context, BuildJobSpec) error {
	return shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) DeleteJob(context.Context, string) error {
	return shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) TriggerBuild(context.Context, string, map[string]string) (BuildQueueItem, error) {
	return BuildQueueItem{}, shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) GetQueueItem(context.Context, string) (BuildQueueItem, error) {
	return BuildQueueItem{}, shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) GetBuildStatus(context.Context, string, int64) (BuildStatus, error) {
	return BuildStatus{}, shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) ProgressiveText(context.Context, string, int64, int64) (ProgressiveText, error) {
	return ProgressiveText{}, shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) CancelBuild(context.Context, string, int64) error {
	return shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}

func (failingRunner) CancelQueueItem(context.Context, string) error {
	return shared.NewError(shared.CodeFailedPrecondition, "build runner port is required")
}
