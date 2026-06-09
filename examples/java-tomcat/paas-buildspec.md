# Java Tomcat BuildSpec

```text
build_strategy = java_tomcat
source_path = examples/java-tomcat
build_command = mvn clean package -DskipTests
artifact_copy_command = cp -ar target/paas-tomcat-demo.war "$PAAS_ARTIFACT_OUTPUT/app.war"
java_version = 17
runtime_base_image = registry.example/runtime/tomcat8:1.0
default_ref = main
```
