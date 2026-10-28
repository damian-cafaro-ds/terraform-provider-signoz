package provider_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestDashboardEmptyJSONLifecycle(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("set TF_ACC=1 to run the Terraform CLI regression test")
	}
	terraform, err := exec.LookPath("terraform")
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	build := exec.Command("go", "build", "-o", filepath.Join(binDir, "terraform-provider-signoz"), "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, output)
	}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_state_%t", legacy), func(t *testing.T) {
			var mu sync.Mutex
			posts, puts := 0, 0
			data := map[string]interface{}{
				"title": "JSON regression", "description": "before", "name": "", "version": "v5",
				"layout": []interface{}{}, "widgets": []interface{}{}, "tags": []string{"test"},
				"variables": map[string]interface{}{}, "panelMap": map[string]interface{}{},
				"collapsableRowsMigrated": false, "uploadedGrafana": false,
			}
			updatedAt := "2026-01-01T00:00:00Z"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if r.URL.Path != "/api/v1/dashboards" && r.URL.Path != "/api/v1/dashboards/test-dashboard" {
					http.NotFound(w, r)
					return
				}
				switch r.Method {
				case http.MethodGet:
				case http.MethodPost, http.MethodPut:
					if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
						t.Errorf("decode dashboard: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if _, ok := data["variables"].(map[string]interface{}); !ok {
						t.Errorf("variables must be an object, got %T", data["variables"])
					}
					delete(data, "panelMap")
					if r.Method == http.MethodPost {
						posts++
					} else {
						puts++
						updatedAt = "2026-01-02T00:00:00Z"
					}
				default:
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success", "data": map[string]interface{}{
						"id": "test-dashboard", "data": data, "createdAt": "2026-01-01T00:00:00Z",
						"createdBy": "test", "updatedAt": updatedAt, "updatedBy": "test",
					},
				})
			}))
			defer server.Close()
			data["source"] = server.URL + "/dashboard"
			dir := t.TempDir()
			cliConfig := filepath.Join(dir, "terraform.tfrc")
			if err := os.WriteFile(cliConfig, []byte(fmt.Sprintf("provider_installation {\n dev_overrides {\n  \"registry.terraform.io/signoz/signoz\" = %q\n }\n}\n", binDir)), 0600); err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command(terraform, append([]string{"-chdir=" + dir}, args...)...)
				cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+cliConfig, "CHECKPOINT_DISABLE=1", "TF_IN_AUTOMATION=true")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("terraform %v: %v\n%s", args, err, output)
				}
			}
			resource := map[string]interface{}{
				"title": "JSON regression", "description": "before", "name": "", "version": "v5",
				"layout": "[]", "widgets": "[]", "tags": []string{"test"}, "variables": "{}",
				"collapsable_rows_migrated": false, "uploaded_grafana": false,
			}
			if legacy {
				resource["lifecycle"] = map[string]interface{}{"ignore_changes": []string{"variables"}}
			}
			writeConfig := func() {
				t.Helper()
				config := map[string]interface{}{
					"terraform": map[string]interface{}{"required_providers": map[string]interface{}{"signoz": map[string]interface{}{"source": "signoz/signoz"}}},
					"provider":  map[string]interface{}{"signoz": map[string]interface{}{"endpoint": server.URL, "access_token": "local-test", "http_max_retry": 0}},
					"resource":  map[string]interface{}{"signoz_dashboard": map[string]interface{}{"test": resource}},
				}
				payload, err := json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), payload, 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeConfig()
			if legacy {
				run("import", "-input=false", "signoz_dashboard.test", "test-dashboard")
				statePath := filepath.Join(dir, "terraform.tfstate")
				payload, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatal(err)
				}
				decode := func(raw []byte, target interface{}) {
					t.Helper()
					if err := json.Unmarshal(raw, target); err != nil {
						t.Fatal(err)
					}
				}
				encode := func(value interface{}) json.RawMessage {
					t.Helper()
					result, err := json.Marshal(value)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				var state map[string]json.RawMessage
				var resources, instances []map[string]json.RawMessage
				var attrs map[string]interface{}
				decode(payload, &state)
				decode(state["resources"], &resources)
				decode(resources[0]["instances"], &instances)
				decode(instances[0]["attributes"], &attrs)
				attrs["variables"], attrs["panel_map"] = "", ""
				instances[0]["attributes"] = encode(attrs)
				resources[0]["instances"] = encode(instances)
				state["resources"] = encode(resources)
				if err := os.WriteFile(statePath, encode(state), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				run("apply", "-auto-approve", "-input=false")
			}
			run("plan", "-detailed-exitcode", "-input=false")
			resource["description"] = "after"
			writeConfig()
			run("apply", "-auto-approve", "-input=false")
			for i := 0; i < 2; i++ {
				run("plan", "-detailed-exitcode", "-input=false")
			}
			mu.Lock()
			defer mu.Unlock()
			wantPosts := 1
			if legacy {
				wantPosts = 0
			}
			if posts != wantPosts || puts != 1 || data["description"] != "after" {
				t.Fatalf("requests: POST=%d PUT=%d description=%v", posts, puts, data["description"])
			}
		})
	}
}
