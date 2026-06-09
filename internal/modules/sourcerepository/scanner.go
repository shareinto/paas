package sourcerepository

import (
	"path"
	"sort"
	"strings"

	"github.com/shareinto/paas/internal/shared"
)

const (
	defaultSpringBootRuntimeBaseImage = "paas-runtime/java-springboot:17"
	defaultTomcatRuntimeBaseImage     = "paas-runtime/java-tomcat:17"
)

func GenerateJavaBuildSpecSuggestions(files []RepositoryFile) []BuildSpecSuggestion {
	paths := make([]string, 0, len(files))
	seen := map[string]struct{}{}
	for _, file := range files {
		normalized := cleanRepositoryPath(file.Path)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		paths = append(paths, normalized)
	}
	sort.Strings(paths)

	buildRoots := map[string]string{}
	jarsByRoot := map[string][]string{}
	warsByRoot := map[string][]string{}
	for _, filePath := range paths {
		switch {
		case strings.HasSuffix(filePath, "/pom.xml") || filePath == "pom.xml":
			buildRoots[sourcePathForBuildFile(filePath, "pom.xml")] = "maven"
		case strings.HasSuffix(filePath, "/build.gradle") || filePath == "build.gradle":
			buildRoots[sourcePathForBuildFile(filePath, "build.gradle")] = "gradle"
		case strings.HasSuffix(filePath, ".jar") && strings.Contains(filePath, "/target/"):
			root := strings.TrimSuffix(strings.Split(filePath, "/target/")[0], "/")
			jarsByRoot[root] = append(jarsByRoot[root], filePath)
		case strings.HasSuffix(filePath, ".war") && strings.Contains(filePath, "/target/"):
			root := strings.TrimSuffix(strings.Split(filePath, "/target/")[0], "/")
			warsByRoot[root] = append(warsByRoot[root], filePath)
		}
	}

	rootSet := map[string]struct{}{}
	for root := range buildRoots {
		rootSet[root] = struct{}{}
	}
	for root := range jarsByRoot {
		rootSet[root] = struct{}{}
	}
	for root := range warsByRoot {
		rootSet[root] = struct{}{}
	}
	roots := make([]string, 0, len(rootSet))
	for root := range rootSet {
		roots = append(roots, root)
	}
	sort.Strings(roots)

	suggestions := make([]BuildSpecSuggestion, 0)
	for _, root := range roots {
		tool := buildRoots[root]
		command := buildCommand(tool)
		if command == "" {
			command = "mvn clean package -DskipTests"
		}
		if jars := sortedCopy(jarsByRoot[root]); len(jars) > 0 {
			suggestions = append(suggestions, BuildSpecSuggestion{
				SourcePath:          root,
				BuildCommand:        command,
				ArtifactCopyCommand: artifactCopyCommand(root, jars[0], "app.jar"),
				RuntimeBaseImage:    defaultSpringBootRuntimeBaseImage,
				Evidence:            evidenceFor(root, tool, jars[0]),
			})
		}
		if wars := sortedCopy(warsByRoot[root]); len(wars) > 0 {
			suggestions = append(suggestions, BuildSpecSuggestion{
				SourcePath:          root,
				BuildCommand:        command,
				ArtifactCopyCommand: artifactCopyCommand(root, wars[0], "app.war"),
				RuntimeBaseImage:    defaultTomcatRuntimeBaseImage,
				Evidence:            evidenceFor(root, tool, wars[0]),
			})
		}
		if len(jarsByRoot[root]) == 0 && len(warsByRoot[root]) == 0 && tool != "" {
			suggestions = append(suggestions, BuildSpecSuggestion{
				SourcePath:       root,
				BuildCommand:     command,
				RuntimeBaseImage: defaultSpringBootRuntimeBaseImage,
				Evidence:         evidenceFor(root, tool, ""),
			})
		}
	}
	return suggestions
}

func artifactCopyCommand(root string, artifactFilePath string, outputName string) string {
	relative := artifactFilePath
	if root != "." && root != "" {
		relative = strings.TrimPrefix(artifactFilePath, root+"/")
	}
	return `cp -ar ` + relative + ` "$PAAS_ARTIFACT_OUTPUT/` + outputName + `"`
}

func ValidateBuildSpecSuggestion(suggestion BuildSpecSuggestion) error {
	if strings.TrimSpace(suggestion.BuildCommand) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "build_command is required")
	}
	if strings.TrimSpace(suggestion.RuntimeBaseImage) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "runtime_base_image is required")
	}
	return nil
}

func cleanRepositoryPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
		return ""
	}
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return ""
	}
	return cleaned
}

func sourcePathForBuildFile(filePath string, fileName string) string {
	if filePath == fileName {
		return "."
	}
	return strings.TrimSuffix(filePath, "/"+fileName)
}

func buildCommand(tool string) string {
	switch tool {
	case "maven":
		return "mvn clean package -DskipTests"
	case "gradle":
		return "./gradlew clean build -x test"
	default:
		return ""
	}
}

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

func evidenceFor(root string, tool string, artifact string) []string {
	evidence := make([]string, 0, 2)
	switch tool {
	case "maven":
		if root == "." {
			evidence = append(evidence, "pom.xml")
		} else {
			evidence = append(evidence, root+"/pom.xml")
		}
	case "gradle":
		if root == "." {
			evidence = append(evidence, "build.gradle")
		} else {
			evidence = append(evidence, root+"/build.gradle")
		}
	}
	if artifact != "" {
		evidence = append(evidence, artifact)
	}
	return evidence
}
