package cloudstack

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-cloudstack/cloudstack"
	"log"
	"regexp"
	"time"
)

func dataSourceCloudstackTemplate() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCloudstackTemplateRead,
		Schema: map[string]*schema.Schema{
			"filter": dataSourceFiltersSchema(),
			"templatefilter": {
				Type:     schema.TypeString,
				Required: true,
			},

			// Computed values
			"template_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"account": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"created": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"displaytext": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"format": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"hypervisor": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"size": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": tagsSchema(),
		},
	}
}

func dataSourceCloudstackTemplateRead(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)
	ltp := cloudstack.ListTemplatesParams{}
	ltp.SetListall(true)
	ltp.SetTemplatefilter(d.Get("templatefilter").(string))
	csTemplates, err := cs.Template.ListTemplates(&ltp)
	if err != nil {
		log.Printf("[ERROR] Failed to list templates: %s", err)
	}

	filters, filtersOk := d.GetOk("filter")
	var template *cloudstack.Template
	var templates []*cloudstack.Template

	if filtersOk {
		for _, t := range csTemplates.Templates {
			if applyFilters(t, filters.(*schema.Set)) {
				templates = append(templates, t)
			}
		}
	} else {
		return fmt.Errorf("No specified filter, too many results.")
	}

	if len(templates) > 1 {
		template = mostRecentTemplate(templates)
	} else if len(templates) == 1 {
		template = templates[0]
	} else {
		return fmt.Errorf("No template is matching with the specified regex.\n")
	}

	log.Printf("[DEBUG] Selected template: %s\n", template.Displaytext)
	return templateDescriptionAttributes(d, template)
}

func templateDescriptionAttributes(d *schema.ResourceData, template *cloudstack.Template) error {
	d.SetId(template.Id)
	d.Set("template_id", template.Id)
	d.Set("account", template.Account)
	d.Set("created", template.Created)
	d.Set("displaytext", template.Displaytext)
	d.Set("format", template.Format)
	d.Set("hypervisor", template.Hypervisor)
	d.Set("name", template.Name)
	d.Set("size", template.Size)
	d.Set("tags", template.Tags)
	return nil
}

func mostRecentTemplate(templates []*cloudstack.Template) *cloudstack.Template {
	var mrt int
	mostRecent := int64(0)
	for k, t := range templates {
		created, err := time.Parse("2006-01-02T15:04:05-0700", t.Created)
		if err != nil {
			panic(err)
		}

		if created.Unix() > mostRecent {
			mostRecent = created.Unix()
			mrt = k
		}
	}

	return templates[mrt]
}

func applyFilters(template *cloudstack.Template, filters *schema.Set) bool {
	var templateJSON map[string]interface{}
	t, _ := json.Marshal(template)
	json.Unmarshal(t, &templateJSON)

	for _, f := range filters.List() {
		m := f.(map[string]interface{})
		r := regexp.MustCompile(m["value"].(string))
		templateField := templateJSON[m["name"].(string)].(string)
		if !r.Match([]byte(templateField)) {
			return false
		}

	}
	return true
}
