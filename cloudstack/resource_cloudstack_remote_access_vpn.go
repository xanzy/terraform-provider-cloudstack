package cloudstack

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-cloudstack/cloudstack"
)

func resourceCloudStackRemoteAccessVPN() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudStackRemoteAccessVPNCreate,
		Read:   resourceCloudStackRemoteAccessVPNRead,
		Update: resourceCloudStackRemoteAccessVPNUpdate,
		Delete: resourceCloudStackRemoteAccessVPNDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"public_ip_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"account": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"domainid": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"fordisplay": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"iprange": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"openfirewall": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"public_ip": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"domain": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"presharedkey": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"project": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"projectid": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"state": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceCloudStackRemoteAccessVPNCreate(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	publicipid := d.Get("public_ip_id").(string)
	p := cs.VPN.NewCreateRemoteAccessVpnParams(publicipid)

	// Create the new VPN Gateway
	v, err := cs.VPN.CreateRemoteAccessVpn(p)
	if err != nil {
		return fmt.Errorf("Error creating Remote Access VPN for Public IP %s: %s", publicipid, err)
	}

	log.Printf("[DEBUG] Remote Access VPN created: %+v", v)

	for i := 0; i < 12; i++ {
		_, count, err := cs.VPN.GetRemoteAccessVpnByID(v.Id)
		if err != nil {
			if count == 0 {
				time.Sleep(5 * time.Second)
				continue
			}
			return fmt.Errorf("Error looking for newly created Remote Access VPN for Public IP %s: %s", publicipid, err)
		}
	}

	d.SetId(v.Id)

	// log.Printf("[DEBUG] Sleeping for 40 seconds")

	// time.Sleep(40 * time.Second)

	return resourceCloudStackRemoteAccessVPNRead(d, meta)
}

func resourceCloudStackRemoteAccessVPNRead(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	// Get the VPN Gateway details
	v, count, err := cs.VPN.GetRemoteAccessVpnByID(d.Id(), cloudstack.WithProject(d.Get("project").(string)))
	log.Printf("[DEBUG] count is %d", count)
	log.Printf("[DEBUG] err is %s", err)
	if err != nil {
		if count == 0 {
			log.Printf(
				"[DEBUG] Remote Access VPN for public IP %s does no longer exist", d.Get("public_ip_id").(string))
			d.SetId("")
			return nil
		}

		return err
	}

	d.Set("account", v.Account)
	d.Set("domain", v.Domain)
	d.Set("domainid", v.Domainid)
	d.Set("fordisplay", v.Fordisplay)
	d.Set("iprange", v.Iprange)
	d.Set("presharedkey", v.Presharedkey)
	d.Set("project", v.Project)
	d.Set("projectid", v.Projectid)
	d.Set("public_ip", v.Publicip)
	d.Set("state", v.State)

	return nil
}

func resourceCloudStackRemoteAccessVPNUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourceCloudStackRemoteAccessVPNRead(d, meta)
}

func resourceCloudStackRemoteAccessVPNDelete(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	// Create a new parameter struct
	p := cs.VPN.NewDeleteRemoteAccessVpnParams(d.Id())

	// Delete the Remote Access VPN
	_, err := cs.VPN.DeleteRemoteAccessVpn(p)
	if err != nil {
		// This is a very poor way to be told the ID does no longer exist :(
		if strings.Contains(err.Error(), fmt.Sprintf(
			"Invalid parameter id value=%s due to incorrect long value format, "+
				"or entity does not exist", d.Id())) {
			return nil
		}

		return fmt.Errorf("Error deleting Remote Access VPN for Public IP %s: %s", d.Get("public_ip_id").(string), err)
	}

	return nil
}
