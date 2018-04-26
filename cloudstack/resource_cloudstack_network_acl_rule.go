package cloudstack

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-cloudstack/cloudstack"
)

func resourceCloudStackNetworkACLRule() *schema.Resource {
	return &schema.Resource{
		Create: resourceCloudStackNetworkACLRuleCreate,
		Read:   resourceCloudStackNetworkACLRuleRead,
		Update: resourceCloudStackNetworkACLRuleUpdate,
		Delete: resourceCloudStackNetworkACLRuleDelete,

		Schema: map[string]*schema.Schema{
			"acl_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"managed": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"rule": &schema.Schema{
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"action": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Default:  "allow",
						},

						"cidr_list": &schema.Schema{
							Type:     schema.TypeSet,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      schema.HashString,
						},

						"protocol": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},

						"icmp_type": &schema.Schema{
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
						},

						"icmp_code": &schema.Schema{
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
						},

						"ports": &schema.Schema{
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							Set:      schema.HashString,
						},

						"traffic_type": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Default:  "ingress",
						},

						"uuids": &schema.Schema{
							Type:     schema.TypeMap,
							Computed: true,
						},
					},
				},
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"parallelism": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  2,
			},
		},
	}
}

func resourceCloudStackNetworkACLRuleCreate(d *schema.ResourceData, meta interface{}) error {
	// Make sure all required parameters are there
	if err := verifyNetworkACLParams(d); err != nil {
		return err
	}

	// We need to set this upfront in order to be able to save a partial state
	d.SetId(d.Get("acl_id").(string))

	// Create all rules that are configured
	if nrs := d.Get("rule").([]interface{}); len(nrs) > 0 {
		// Create an empty list to hold all newly created rules
		var rules []interface{}

		err := createNetworkACLRules(d, meta, &rules, nrs)

		// We need to update this first to preserve the correct state
		d.Set("rule", rules)

		if err != nil {
			return err
		}
	}

	return resourceCloudStackNetworkACLRuleRead(d, meta)
}

var index int

func createNetworkACLRules(d *schema.ResourceData, meta interface{}, rules *[]interface{}, nrs []interface{}) error {
	var errs *multierror.Error

	var wg sync.WaitGroup
	wg.Add(len(nrs))

	index = 1
	sem := make(chan struct{}, d.Get("parallelism").(int))
	for _, rule := range nrs {
		rule := rule.(map[string]interface{})

		// Put in a tiny sleep here to avoid DoS'ing the API
		time.Sleep(500 * time.Millisecond)

		go func(index int, rule map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}

			// Create a single rule
			err := createNetworkACLRule(d, meta, index, rule)

			// If we have at least one UUID, we need to save the rule
			if len(rule["uuids"].(map[string]interface{})) > 0 {
				*rules = append(*rules, rule)
			}

			if err != nil {
				errs = multierror.Append(errs, err)
			}

			<-sem
		}(index, rule)

		index += rule["ports"].(*schema.Set).Len()
	}

	wg.Wait()

	return errs.ErrorOrNil()
}

func createNetworkACLRule(d *schema.ResourceData, meta interface{}, index int, rule map[string]interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)
	uuids := rule["uuids"].(map[string]interface{})

	// Make sure all required parameters are there
	if err := verifyNetworkACLRuleParams(d, rule); err != nil {
		return err
	}

	// Create a new parameter struct
	p := cs.NetworkACL.NewCreateNetworkACLParams(rule["protocol"].(string))

	// Set the acl ID
	p.SetAclid(d.Id())

	// Set the action
	p.SetAction(rule["action"].(string))

	// Set the CIDR list
	var cidrList []string
	for _, cidr := range rule["cidr_list"].(*schema.Set).List() {
		cidrList = append(cidrList, cidr.(string))
	}
	p.SetCidrlist(cidrList)

	// Set the traffic type
	p.SetTraffictype(rule["traffic_type"].(string))

	// If the protocol is ICMP set the needed ICMP parameters
	if rule["protocol"].(string) == "icmp" {
		p.SetNumber(index)
		p.SetIcmptype(rule["icmp_type"].(int))
		p.SetIcmpcode(rule["icmp_code"].(int))

		r, err := Retry(4, retryableACLCreationFunc(cs, p))
		if err != nil {
			return err
		}

		uuids["icmp"] = r.(*cloudstack.CreateNetworkACLResponse).Id
		rule["uuids"] = uuids
	}

	// If the protocol is ALL set the needed parameters
	if rule["protocol"].(string) == "all" {
		p.SetNumber(index)

		r, err := Retry(4, retryableACLCreationFunc(cs, p))
		if err != nil {
			return err
		}

		uuids["all"] = r.(*cloudstack.CreateNetworkACLResponse).Id
		rule["uuids"] = uuids
	}

	// If protocol is TCP or UDP, loop through all ports
	if rule["protocol"].(string) == "tcp" || rule["protocol"].(string) == "udp" {
		if ps := rule["ports"].(*schema.Set); ps.Len() > 0 {

			// Create an empty schema.Set to hold all processed ports
			ports := &schema.Set{F: schema.HashString}

			for i, port := range ps.List() {
				if _, ok := uuids[port.(string)]; ok {
					ports.Add(port)
					rule["ports"] = ports
					continue
				}

				m := splitPorts.FindStringSubmatch(port.(string))

				startPort, err := strconv.Atoi(m[1])
				if err != nil {
					return err
				}

				endPort := startPort
				if m[2] != "" {
					endPort, err = strconv.Atoi(m[2])
					if err != nil {
						return err
					}
				}

				p.SetNumber(index + i)
				p.SetStartport(startPort)
				p.SetEndport(endPort)

				r, err := Retry(4, retryableACLCreationFunc(cs, p))
				if err != nil {
					return err
				}

				ports.Add(port)
				rule["ports"] = ports

				uuids[port.(string)] = r.(*cloudstack.CreateNetworkACLResponse).Id
				rule["uuids"] = uuids
			}
		}
	}

	return nil
}

func resourceCloudStackNetworkACLRuleRead(d *schema.ResourceData, meta interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)

	// First check if the ACL itself still exists
	_, count, err := cs.NetworkACL.GetNetworkACLListByID(
		d.Id(),
		cloudstack.WithProject(d.Get("project").(string)),
	)
	if err != nil {
		if count == 0 {
			log.Printf(
				"[DEBUG] Network ACL list %s does no longer exist", d.Id())
			d.SetId("")
			return nil
		}

		return err
	}

	// Get all the rules from the running environment
	p := cs.NetworkACL.NewListNetworkACLsParams()
	p.SetAclid(d.Id())
	p.SetListall(true)

	l, err := cs.NetworkACL.ListNetworkACLs(p)
	if err != nil {
		return err
	}

	// Make a map of all the rules so we can easily find a rule
	ruleMap := make(map[string]*cloudstack.NetworkACL, l.Count)
	for _, r := range l.NetworkACLs {
		ruleMap[r.Id] = r
	}

	// Create an empty list to hold all rules
	var rules []interface{}

	// Read all rules that are configured
	if rs := d.Get("rule").([]interface{}); len(rs) > 0 {
		for _, rule := range rs {
			rule, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}

			if rule["protocol"].(string) == "icmp" {
				id, ok := rule["uuids"].(map[string]interface{})["icmp"]
				if !ok {
					continue
				}

				// Get the rule
				r, ok := ruleMap[id.(string)]
				if !ok {
					continue
				}

				// Delete the known rule so only unknown rules remain in the ruleMap
				delete(ruleMap, id.(string))

				// Create a set with all CIDR's
				cidrs := &schema.Set{F: schema.HashString}
				for _, cidr := range strings.Split(r.Cidrlist, ",") {
					cidrs.Add(cidr)
				}

				// Update the values
				rule["action"] = strings.ToLower(r.Action)
				rule["cidr_list"] = cidrs
				rule["protocol"] = r.Protocol
				rule["icmp_type"] = r.Icmptype
				rule["icmp_code"] = r.Icmpcode
				rule["traffic_type"] = strings.ToLower(r.Traffictype)
				rule["uuids"] = map[string]interface{}{"icmp": id}
				rules = append(rules, rule)
			}

			if rule["protocol"].(string) == "all" {
				id, ok := rule["uuids"].(map[string]interface{})["all"]
				if !ok {
					continue
				}

				// Get the rule
				r, ok := ruleMap[id.(string)]
				if !ok {
					continue
				}

				// Delete the known rule so only unknown rules remain in the ruleMap
				delete(ruleMap, id.(string))

				// Create a set with all CIDR's
				cidrs := &schema.Set{F: schema.HashString}
				for _, cidr := range strings.Split(r.Cidrlist, ",") {
					cidrs.Add(cidr)
				}

				// Update the values
				rule["action"] = strings.ToLower(r.Action)
				rule["cidr_list"] = cidrs
				rule["protocol"] = r.Protocol
				rule["traffic_type"] = strings.ToLower(r.Traffictype)
				rule["uuids"] = map[string]interface{}{"all": id}
				rules = append(rules, rule)
			}

			// If protocol is tcp or udp, loop through all ports
			if rule["protocol"].(string) == "tcp" || rule["protocol"].(string) == "udp" {
				if ps := rule["ports"].(*schema.Set); ps.Len() > 0 {

					// Create an empty schema.Set to hold all ports
					ports := &schema.Set{F: schema.HashString}

					// Create an empty map to hold all the UUIDs
					uuids := make(map[string]interface{})

					// Loop through all ports and retrieve their info
					for _, port := range ps.List() {
						id, ok := rule["uuids"].(map[string]interface{})[port.(string)]
						if !ok {
							continue
						}

						// Get the rule
						r, ok := ruleMap[id.(string)]
						if !ok {
							continue
						}

						// Delete the known rule so only unknown rules remain in the ruleMap
						delete(ruleMap, id.(string))

						// Create a set with all CIDR's
						cidrs := &schema.Set{F: schema.HashString}
						for _, cidr := range strings.Split(r.Cidrlist, ",") {
							cidrs.Add(cidr)
						}

						// Update the values
						rule["action"] = strings.ToLower(r.Action)
						rule["cidr_list"] = cidrs
						rule["protocol"] = r.Protocol
						rule["traffic_type"] = strings.ToLower(r.Traffictype)
						ports.Add(port)
						uuids[port.(string)] = id
					}

					// If there is at least one port found, add this rule to the rules set
					if ports.Len() > 0 {
						rule["ports"] = ports
						rule["uuids"] = uuids
						rules = append(rules, rule)
					}
				}
			}
		}
	}

	// If this is a managed firewall, add all unknown rules into dummy rules
	managed := d.Get("managed").(bool)
	if managed && len(ruleMap) > 0 {
		for uuid := range ruleMap {
			// We need to create and add a dummy value to a schema.Set as the
			// cidr_list is a required field and thus needs a value
			cidrs := &schema.Set{F: schema.HashString}
			cidrs.Add(uuid)

			// Make a dummy rule to hold the unknown UUID
			rule := map[string]interface{}{
				"cidr_list": cidrs,
				"protocol":  uuid,
				"uuids":     map[string]interface{}{uuid: uuid},
			}

			// Add the dummy rule to the rules set
			rules = append(rules, rule)
		}
	}

	if len(rules) > 0 {
		d.Set("rule", rules)
	} else if !managed {
		d.SetId("")
	}

	return nil
}

func resourceCloudStackNetworkACLRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	// Make sure all required parameters are there
	if err := verifyNetworkACLParams(d); err != nil {
		return err
	}

	// Check if the rule set as a whole has changed
	if d.HasChange("rule") {
		o, n := d.GetChange("rule")
		ors := difference(o.([]interface{}), n.([]interface{}))
		nrs := difference(n.([]interface{}), o.([]interface{}))

		// We need to start with a list containing all the rules we
		// already have and want to keep. Any rules that are not deleted
		// correctly and any newly created rules, will be added to this
		// list to make sure we end up in a consistent state
		rules := intersection(o.([]interface{}), n.([]interface{}))

		// First loop over all the old rules and delete them
		if len(ors) > 0 {
			err := deleteNetworkACLRules(d, meta, &rules, ors)

			// We need to update this first to preserve the correct state
			d.Set("rule", rules)

			if err != nil {
				return err
			}
		}

		// Then loop over all the new rules and create them
		if len(nrs) > 0 {
			err := createNetworkACLRules(d, meta, &rules, nrs)

			// We need to update this first to preserve the correct state
			d.Set("rule", rules)

			if err != nil {
				return err
			}
		}
	}

	return resourceCloudStackNetworkACLRuleRead(d, meta)
}

func difference(s1, s2 []interface{}) []interface{} {
	rs := resourceCloudStackNetworkACLRule().Schema["rule"].Elem.(*schema.Resource)
	hash := schema.HashResource(rs)

	result := s1[:0]
	for i, elem := range s1 {
		if i < len(s2) {
			if hash(elem) == hash(s2[i]) {
				log.Printf("SVH8 NO MATCH:\n%#+v\n%#+v", elem, s2[i])
				result = append(result, elem)
			} else {
				log.Printf("SVH8 MATCH:\n%#+v\n%#+v", elem, s2[i])
			}
		}
	}
	return result
}

func intersection(s1, s2 []interface{}) []interface{} {
	result := s1[:0]
	for i, elem := range s1 {
		if i < len(s2) {
			if reflect.DeepEqual(elem, s2[i]) {
				result = append(result, elem)
			}
		}
	}
	// log.Printf("SVH9:\n%#+v\n%#+v\n%#+v", s1, s2, result)
	return result

}

func resourceCloudStackNetworkACLRuleDelete(d *schema.ResourceData, meta interface{}) error {
	// Delete all rules
	if ors := d.Get("rule").([]interface{}); len(ors) > 0 {
		// Create an empty list to hold all rules that where not deleted correctly
		var rules []interface{}

		err := deleteNetworkACLRules(d, meta, &rules, ors)

		// We need to update this first to preserve the correct state
		d.Set("rule", rules)

		if err != nil {
			return err
		}
	}

	return nil
}

func deleteNetworkACLRules(d *schema.ResourceData, meta interface{}, rules *[]interface{}, ors []interface{}) error {
	var errs *multierror.Error

	var wg sync.WaitGroup
	wg.Add(len(ors))

	sem := make(chan struct{}, d.Get("parallelism").(int))
	for _, rule := range ors {
		// Put a sleep here to avoid DoS'ing the API
		time.Sleep(500 * time.Millisecond)

		go func(rule map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}

			// Delete a single rule
			err := deleteNetworkACLRule(d, meta, rule)

			// If we have at least one UUID, we need to save the rule
			if len(rule["uuids"].(map[string]interface{})) > 0 {
				*rules = append(*rules, rule)
			}

			if err != nil {
				errs = multierror.Append(errs, err)
			}

			<-sem
		}(rule.(map[string]interface{}))
	}

	wg.Wait()

	return errs.ErrorOrNil()
}

func deleteNetworkACLRule(d *schema.ResourceData, meta interface{}, rule map[string]interface{}) error {
	cs := meta.(*cloudstack.CloudStackClient)
	uuids := rule["uuids"].(map[string]interface{})

	for k, id := range uuids {
		// We don't care about the count here, so just continue
		if k == "%" {
			continue
		}

		// Create the parameter struct
		p := cs.NetworkACL.NewDeleteNetworkACLParams(id.(string))

		// Delete the rule
		if _, err := cs.NetworkACL.DeleteNetworkACL(p); err != nil {

			// This is a very poor way to be told the ID does no longer exist :(
			if strings.Contains(err.Error(), fmt.Sprintf(
				"Invalid parameter id value=%s due to incorrect long value format, "+
					"or entity does not exist", id.(string))) {
				delete(uuids, k)
				rule["uuids"] = uuids
				continue
			}

			return err
		}

		// Delete the UUID of this rule
		delete(uuids, k)
		rule["uuids"] = uuids
	}

	return nil
}

func verifyNetworkACLParams(d *schema.ResourceData) error {
	managed := d.Get("managed").(bool)
	_, rules := d.GetOk("rule")

	if !rules && !managed {
		return fmt.Errorf(
			"You must supply at least one 'rule' when not using the 'managed' firewall feature")
	}

	return nil
}

func verifyNetworkACLRuleParams(d *schema.ResourceData, rule map[string]interface{}) error {
	action := rule["action"].(string)
	if action != "allow" && action != "deny" {
		return fmt.Errorf("Parameter action only accepts 'allow' or 'deny' as values")
	}

	protocol := rule["protocol"].(string)
	switch protocol {
	case "icmp":
		if _, ok := rule["icmp_type"]; !ok {
			return fmt.Errorf(
				"Parameter icmp_type is a required parameter when using protocol 'icmp'")
		}
		if _, ok := rule["icmp_code"]; !ok {
			return fmt.Errorf(
				"Parameter icmp_code is a required parameter when using protocol 'icmp'")
		}
	case "all":
		// No additional test are needed, so just leave this empty...
	case "tcp", "udp":
		if ports, ok := rule["ports"].(*schema.Set); ok {
			for _, port := range ports.List() {
				m := splitPorts.FindStringSubmatch(port.(string))
				if m == nil {
					return fmt.Errorf(
						"%q is not a valid port value. Valid options are '80' or '80-90'", port.(string))
				}
			}
		} else {
			return fmt.Errorf(
				"Parameter ports is a required parameter when *not* using protocol 'icmp'")
		}
	default:
		_, err := strconv.ParseInt(protocol, 0, 0)
		if err != nil {
			return fmt.Errorf(
				"%q is not a valid protocol. Valid options are 'tcp', 'udp', "+
					"'icmp', 'all' or a valid protocol number", protocol)
		}
	}

	traffic := rule["traffic_type"].(string)
	if traffic != "ingress" && traffic != "egress" {
		return fmt.Errorf(
			"Parameter traffic_type only accepts 'ingress' or 'egress' as values")
	}

	return nil
}

func retryableACLCreationFunc(
	cs *cloudstack.CloudStackClient,
	p *cloudstack.CreateNetworkACLParams) func() (interface{}, error) {
	return func() (interface{}, error) {
		r, err := cs.NetworkACL.CreateNetworkACL(p)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
}
