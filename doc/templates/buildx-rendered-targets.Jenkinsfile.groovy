pipeline {
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
          sh '''
            set -eu
            repo_url='{{ .RepoURL }}'
            ref='{{ .GitRef }}'
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
              primary_commit="$(cat report/source-{{ $.PrimarySourceKey }}-commit.txt 2>/dev/null || true)"
              image_tag_commit="$(printf '%s' "$primary_commit" | cut -c1-8)"
              if [ -z "$image_tag_commit" ]; then
                image_tag_commit='{{ $.ImageTagFallback }}'
              fi
              image_uri='{{ .ImageRepository }}:{{ $.ImageTagDate }}-'"${image_tag_commit}"'-{{ .Key }}'
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
              selector_labels: new groovy.json.JsonSlurperClassic().parseText('''{{ .SelectorLabelsJSON }}'''),
              metadata: new groovy.json.JsonSlurperClassic().parseText('''{{ .MetadataJSON }}''')
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
}
