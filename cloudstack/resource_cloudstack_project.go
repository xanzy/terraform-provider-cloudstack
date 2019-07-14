package cloudstack

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-cloudstack/cloudstack"
)

func resourceCloudStackProject() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudStackProjectCreate,
		Read:   resourceCloudStackProjectRead,
		Update: resourceCloudStackProjectUpdate,
		Delete: resourceCloudStackProjectDelete,
		Importer: &schema.ResourceImporter{
			State: importStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"display_text": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"account": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"domain_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceCloudStackProjectCreate(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	// Get the name from the config
	name := d.Get("name").(string)

	// Set the display text
	displaytext, ok := d.GetOk("display_text")
	if !ok {
		displaytext = name
	}

	// Create a new parameter struct
	p := cs.Project.NewCreateProjectParams(
		displaytext.(string),
		name,
	)

	// If there is a account supplied, make sure to add it to the request
	if account, ok := d.GetOk("account"); ok {
		// Set the account
		p.SetAccount(account.(string))
	}

	// If there is a domain id supplied, make sure to add it to the request
	if domainid, ok := d.GetOk("domain_id"); ok {
		// Set the domain id
		p.SetDomainid(domainid.(string))
	}

	// Create the new project
	r, err := cs.Project.CreateProject(p)
	if err != nil {
		return fmt.Errorf("Error creating the new project %s: %s", name, err)
	}

	d.SetId(r.Id)

	// Set tags if necessary
	err = setTags(cs, d, "Project")
	if err != nil {
		return fmt.Errorf("Error setting tags on the Project: %s", err)
	}

	return resourceCloudStackProjectRead(d, meta)
}

func resourceCloudStackProjectRead(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	p, count, err := cs.Project.GetProjectByID(
		d.Id(),
		cloudstack.WithDomain(d.Get("domain_id").(string)),
	)
	if err != nil {
		if count == 0 {
			log.Printf(
				"[DEBUG] Project %s does no longer exist", d.Get("name").(string))
			d.SetId("")
			return nil
		}

		return err
	}

	d.Set("name", p.Name)
	d.Set("display_text", p.Displaytext)
	d.Set("account", p.Account)
	d.Set("domain_id", p.Domainid)

	tags := make(map[string]interface{})
	for _, tag := range p.Tags {
		tags[tag.Key] = tag.Value
	}
	d.Set("tags", tags)

	return nil
}

func resourceCloudStackProjectUpdate(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	name := d.Get("name").(string)

	// Check if the display text is changed
	if d.HasChange("display_text") {
		// Create a new parameter struct
		p := cs.Project.NewUpdateProjectParams(d.Id())

		// Set the display text
		displaytext, ok := d.GetOk("display_text")
		if !ok {
			displaytext = d.Get("name")
		}

		// Set the new display text
		p.SetDisplaytext(displaytext.(string))

		// Update the Project
		_, err := cs.Project.UpdateProject(p)
		if err != nil {
			return fmt.Errorf(
				"Error updating display test of Project %s: %s", name, err)
		}
	}

	// Check is the tags have changed
	if d.HasChange("tags") {
		err := updateTags(cs, d, "Project")
		if err != nil {
			return fmt.Errorf("Error updating tags on Project %s: %s", name, err)
		}
		d.SetPartial("tags")
	}

	return resourceCloudStackProjectRead(d, meta)
}

func resourceCloudStackProjectDelete(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	// Create a new parameter struct
	p := cs.Project.NewDeleteProjectParams(d.Id())

	// Delete the Project
	_, err := cs.Project.DeleteProject(p)
	if err != nil {
		// This is a very poor way to be told the ID does no longer exist :(
		if strings.Contains(err.Error(), fmt.Sprintf(
			"Invalid parameter id value=%s due to incorrect long value format, "+
				"or entity does not exist", d.Id())) {
			return nil
		}

		return fmt.Errorf("Error deleting Project %s: %s", d.Get("name").(string), err)
	}

	return nil
}
