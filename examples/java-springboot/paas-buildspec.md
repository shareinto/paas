# Java Spring Boot BuildSpec

```text
build_strategy = java_springboot
source_path = examples/java-springboot
build_command = mvn clean package -DskipTests
artifact_copy_command = cp -ar target/paas-springboot-demo.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"
java_version = 17
runtime_base_image = registry.example/runtime/java17:1.0
default_ref = main
```
