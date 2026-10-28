package model

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/SigNoz/terraform-provider-signoz/signoz/internal/utils"
	tfattr "github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/helper/structure"
)

// Dashboard model.
type Dashboard struct {
	CollapsableRowsMigrated bool                     `json:"collapsableRowsMigrated"`
	Description             string                   `json:"description"`
	Layout                  []map[string]interface{} `json:"layout"`
	Name                    string                   `json:"name"`
	PanelMap                map[string]interface{}   `json:"panelMap,omitempty"`
	Source                  string                   `json:"source"`
	Tags                    []string                 `json:"tags"`
	Title                   string                   `json:"title"`
	UploadedGrafana         bool                     `json:"uploadedGrafana"`
	Variables               map[string]interface{}   `json:"variables"`
	Version                 string                   `json:"version,omitempty"`
	Widgets                 []map[string]interface{} `json:"widgets"`
}

func (d Dashboard) PanelMapToTerraform(previous types.String) (types.String, error) {
	if preserved, ok := preserveDashboardJSON(d.PanelMap, previous); ok {
		return preserved, nil
	}
	if len(d.PanelMap) == 0 {
		return types.StringValue(""), nil
	}
	panelMap, err := structure.FlattenJsonToString(d.PanelMap)
	if err != nil {
		return types.StringNull(), err
	}

	return types.StringValue(panelMap), nil
}

func (d Dashboard) VariablesToTerraform(previous types.String) (types.String, error) {
	if preserved, ok := preserveDashboardJSON(d.Variables, previous); ok {
		return preserved, nil
	}
	if len(d.Variables) == 0 {
		return types.StringValue("{}"), nil
	}
	variables, err := structure.FlattenJsonToString(d.Variables)
	if err != nil {
		return types.StringValue(""), err
	}

	return types.StringValue(variables), nil
}

func preserveDashboardJSON(actual map[string]interface{}, previous types.String) (types.String, bool) {
	if previous.IsNull() || previous.IsUnknown() {
		return types.StringNull(), false
	}
	var prior Dashboard
	if err := prior.SetVariables(previous); err != nil {
		return types.StringNull(), false
	}
	if len(prior.Variables) == 0 && len(actual) == 0 || reflect.DeepEqual(prior.Variables, actual) {
		// Terraform requires the planned representation, including legacy empty strings.
		return previous, true
	}
	return types.StringNull(), false
}

func (d Dashboard) TagsToTerraform() (types.List, diag.Diagnostics) {
	tags := utils.Map(d.Tags, func(value string) tfattr.Value {
		return types.StringValue(value)
	})

	return types.ListValue(types.StringType, tags)
}

func (d Dashboard) LayoutToTerraform() (types.String, error) {
	b, err := json.Marshal(d.Layout)
	if err != nil {
		return types.StringValue(""), err
	}
	return types.StringValue(string(b)), nil
}

func (d Dashboard) WidgetsToTerraform() (types.String, error) {
	b, err := json.Marshal(d.Widgets)
	if err != nil {
		return types.StringValue(""), err
	}
	return types.StringValue(string(b)), nil
}

func (d *Dashboard) SetVariables(tfVariables types.String) error {
	// Older provider state represents an empty variables object as an empty string.
	if strings.TrimSpace(tfVariables.ValueString()) == "" {
		d.Variables = make(map[string]interface{})
		return nil
	}
	variables, err := structure.ExpandJsonFromString(tfVariables.ValueString())
	if err != nil {
		return err
	}
	if variables == nil {
		variables = make(map[string]interface{})
	}
	d.Variables = variables
	return nil
}

func (d *Dashboard) SetPanelMap(tfPanelMap types.String) error {
	if tfPanelMap.ValueString() == "" {
		d.PanelMap = make(map[string]interface{})
		return nil
	}
	panelMap, err := structure.ExpandJsonFromString(tfPanelMap.ValueString())
	if err != nil {
		return err
	}
	d.PanelMap = panelMap
	return nil
}

func (d *Dashboard) SetTags(tfTags types.List) {
	tags := utils.Map(tfTags.Elements(), func(value tfattr.Value) string {
		return strings.Trim(value.String(), "\"")
	})
	d.Tags = tags
}

func (d *Dashboard) SetLayout(tfLayout types.String) error {
	var layout []map[string]interface{}
	err := json.Unmarshal([]byte(tfLayout.ValueString()), &layout)
	if err != nil {
		return err
	}
	d.Layout = layout
	return nil
}

func (d *Dashboard) SetWidgets(tfWidgets types.String) error {
	var widgets []map[string]interface{}
	err := json.Unmarshal([]byte(tfWidgets.ValueString()), &widgets)
	if err != nil {
		return err
	}
	d.Widgets = widgets
	return nil
}

func (d *Dashboard) SetSourceIfEmpty(hostURL string) {
	d.Source = utils.WithDefault(d.Source, hostURL+"/dashboard")
}
