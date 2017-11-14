package cloudstack

import (
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
			"displaytext_regex": {
				Type:     schema.TypeString,
				Required: true,
			},
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
	r := regexp.MustCompile(d.Get("displaytext_regex").(string))
	var templates []*cloudstack.Template

	for _, t := range csTemplates.Templates {
		if r.Match([]byte(t.Displaytext)) {
			templates = append(templates, t)
		}
	}

	var template *cloudstack.Template

	if len(templates) > 1 {
		template = mostRecentTemplate(templates)
	} else if len(templates) == 1 {
		template = templates[0]
	} else {
		return fmt.Errorf("No template is matching with the specified regex.\n")
	}
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

	log.Printf("[DEBUG] Most recent template selected: %+v\n", templates[mrt])
	return templates[mrt]
}
