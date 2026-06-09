package main

import "testing"

func TestMainDoesNotPanic(t *testing.T) {
	t.Setenv("PAAS_CLUSTER_ID", "cluster_1")
	t.Setenv("PAAS_CONTROL_PLANE_URL", "https://paas.example")
	t.Setenv("PAAS_AGENT_TOKEN", "token")
	t.Setenv("PAAS_AGENT_NAMESPACES", " apps, argocd ,,")
	t.Setenv("PAAS_HEARTBEAT_INTERVAL", "5s")
	t.Setenv("PAAS_SNAPSHOT_INTERVAL", "15s")
	main()
}

func TestSplitCSVTrimsAndSkipsBlankParts(t *testing.T) {
	values := splitCSV(" dev, test ,, prod ")
	if len(values) != 3 || values[0] != "dev" || values[2] != "prod" {
		t.Fatalf("unexpected values: %#v", values)
	}
	if values := splitCSV("  "); values != nil {
		t.Fatalf("blank input should return nil: %#v", values)
	}
}
