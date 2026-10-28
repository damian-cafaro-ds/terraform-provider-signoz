package model

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDashboardEmptyVariables(t *testing.T) {
	for _, value := range []string{"", "{}", " { } ", "null"} {
		t.Run(value, func(t *testing.T) {
			dashboard := Dashboard{}
			if err := dashboard.SetVariables(types.StringValue(value)); err != nil {
				t.Fatalf("SetVariables(%q): %v", value, err)
			}
			got, err := dashboard.VariablesToTerraform(types.StringNull())
			if err != nil || got.ValueString() != "{}" {
				t.Fatalf("empty variables = %v, %v; want {}", got, err)
			}
		})
	}
}

func TestDashboardVariablesRoundTrip(t *testing.T) {
	input := `{"region":{"name":"region","selectedValue":"us-east-1","type":"CUSTOM"}}`
	dashboard := Dashboard{}
	if err := dashboard.SetVariables(types.StringValue(input)); err != nil {
		t.Fatal(err)
	}
	got, err := dashboard.VariablesToTerraform(types.StringNull())
	if err != nil || got.ValueString() != input {
		t.Fatalf("variables = %v, %v; want %s", got, err, input)
	}
}

func TestDashboardVariablesRejectInvalidJSON(t *testing.T) {
	for _, value := range []string{"{", "[]", `"text"`} {
		t.Run(value, func(t *testing.T) {
			dashboard := Dashboard{}
			if err := dashboard.SetVariables(types.StringValue(value)); err == nil {
				t.Fatalf("SetVariables(%q) accepted a non-object", value)
			}
		})
	}
}

func TestDashboardEmptyPanelMap(t *testing.T) {
	for _, actual := range []map[string]interface{}{nil, {}} {
		for _, previous := range []types.String{types.StringNull(), types.StringUnknown(), types.StringValue("")} {
			got, err := (Dashboard{PanelMap: actual}).PanelMapToTerraform(previous)
			if err != nil || got.IsNull() || got.IsUnknown() || got.ValueString() != "" {
				t.Fatalf("empty optional panel_map = %v, %v; want empty string", got, err)
			}
		}
	}
	previous := types.StringValue("{}")
	got, err := (Dashboard{}).PanelMapToTerraform(previous)
	if err != nil || !got.Equal(previous) {
		t.Fatalf("configured empty panel_map = %v, %v; want {}", got, err)
	}
}

func TestDashboardVariablesPreserveRepresentation(t *testing.T) {
	tests := []struct {
		name     string
		previous types.String
		actual   map[string]interface{}
		want     string
	}{
		{"legacy empty state", types.StringValue(""), nil, ""},
		{"explicit empty object", types.StringValue("{}"), nil, "{}"},
		{"formatted empty object", types.StringValue(" { } "), map[string]interface{}{}, " { } "},
		{"legacy null JSON", types.StringValue("null"), nil, "null"},
		{"import", types.StringNull(), nil, "{}"},
		{"unknown", types.StringUnknown(), nil, "{}"},
		{"remote removal", types.StringValue(`{"region":"east"}`), nil, "{}"},
		{"remote addition", types.StringValue("{}"), map[string]interface{}{"region": "east"}, `{"region":"east"}`},
		{"same formatted value", types.StringValue(`{ "region": "east" }`), map[string]interface{}{"region": "east"}, `{ "region": "east" }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (Dashboard{Variables: test.actual}).VariablesToTerraform(test.previous)
			if err != nil || got.ValueString() != test.want || got.IsNull() || got.IsUnknown() {
				t.Fatalf("variables = %v, %v; want %q", got, err, test.want)
			}
		})
	}
}
