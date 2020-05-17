package cloudstack

import (
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-cloudstack/v2/cloudstack"
	"log"
)

func dataSourceCloudstackNetwork() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCloudstackNetworkRead,
		Schema: map[string]*schema.Schema{
			"filter": dataSourceFiltersSchema(),

			// Computed values
			"network_id": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"account": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"cidr": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"display_text": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"dns1": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"dns2": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"gateway": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"network_domain": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"network_offering": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"zone": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tags": tagsSchema(),
		},
	}
}

func dataSourceCloudstackNetworkRead(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	p := cloudstack.ListNetworksParams{}
	p.SetListall(true)

	csNetworks, err := cs.Network.ListNetworks(&p)
	if err != nil {
		return fmt.Errorf("Failed to list networks: %s", err)
	}

	filters := d.Get("filter")
	var networks []*cloudstack.Network

	for _, t := range csNetworks.Networks {
		match, err := applyObjectFilters(t, filters.(*schema.Set))
		if err != nil {
			return err
		}

		if match {
			networks = append(networks, t)
		}
	}

	switch len(networks) {
	case 0:
		return fmt.Errorf("No network is matching with the specified regex")
	case 1:
		log.Printf("[DEBUG] Selected network: %s\n", networks[0].Displaytext)
		return networkDescriptionAttributes(d, networks[0])
	default:
		return fmt.Errorf("Too many networks are matching with the specified regex")
	}

}

func networkDescriptionAttributes(d *schema.ResourceData, network *cloudstack.Network) error {
	d.SetId(network.Id)
	d.Set("network_id", network.Id)
	d.Set("account", network.Account)
	d.Set("cidr", network.Cidr)
	d.Set("display_text", network.Displaytext)
	d.Set("dns1", network.Dns1)
	d.Set("dns2", network.Dns2)
	d.Set("gateway", network.Gateway)
	d.Set("name", network.Name)
	d.Set("network_domain", network.Networkdomain)
	d.Set("network_offering", network.Networkofferingname)
	d.Set("zone", network.Zonename)

	tags := make(map[string]interface{})
	for _, tag := range network.Tags {
		tags[tag.Key] = tag.Value
	}
	d.Set("tags", tags)

	return nil
}
