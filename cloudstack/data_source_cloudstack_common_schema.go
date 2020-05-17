package cloudstack

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"regexp"
)

func dataSourceFiltersSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Required: true,
		ForceNew: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Required: true,
				},
				"value": {
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
	}
}

func applyObjectFilters(cloudstackObject interface{}, filters *schema.Set) (bool, error) {
	var objectJSON map[string]interface{}
	t, _ := json.Marshal(cloudstackObject)
	json.Unmarshal(t, &objectJSON)

	for _, f := range filters.List() {
		m := f.(map[string]interface{})

		r, err := regexp.Compile(m["value"].(string))
		if err != nil {
			return false, fmt.Errorf("Invalid regex: %s", err)
		}

		objectField := objectJSON[m["name"].(string)].(string)
		if !r.MatchString(objectField) {
			return false, nil
		}

	}

	return true, nil
}
