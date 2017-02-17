package digitalocean

import (
	"fmt"
	"log"

	"github.com/digitalocean/godo"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceDigitalOceanLoadbalancer() *schema.Resource {
	return &schema.Resource{
		Create: resourceDigitalOceanLoadbalancerCreate,
		Read:   resourceDigitalOceanLoadbalancerRead,
		Update: resourceDigitalOceanLoadbalancerUpdate,
		Delete: resourceDigitalOceanLoadbalancerDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"region": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"algorithm": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "round_robin",
			},

			"forwarding_rule": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"entry_protocol": {
							Type:     schema.TypeString,
							Required: true,
						},
						"entry_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"target_protocol": {
							Type:     schema.TypeString,
							Required: true,
						},
						"target_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"certificate_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"tls_passthrough": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},

			"healthcheck": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"protocol": {
							Type:     schema.TypeString,
							Required: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"path": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"check_interval_seconds": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  10,
						},
						"response_timeout_seconds": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  5,
						},
						"unhealthy_threshold": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  3,
						},
						"healthy_threshold": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  5,
						},
					},
				},
			},

			"sticky_sessions": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "none",
						},
						"cookie_name": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"cookie_ttl_seconds": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},

			"droplet_ids": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Optional: true,
				Computed: true,
				Set:      schema.HashString,
			},

			"redirect_http_to_https": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"ip": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func expandStickySessions(config []interface{}) *godo.StickySessions {
	stickysessionConfig := config[0].(map[string]interface{})

	stickySession := &godo.StickySessions{
		Type: stickysessionConfig["type"].(string),
	}

	if v, ok := stickysessionConfig["cookie_name"]; ok {
		stickySession.CookieName = v.(string)
	}

	if v, ok := stickysessionConfig["cookie_ttl_seconds"]; ok {
		stickySession.CookieTtlSeconds = v.(int)
	}

	return stickySession
}

func expandHealthCheck(config []interface{}) *godo.HealthCheck {
	healthcheckConfig := config[0].(map[string]interface{})

	healthcheck := &godo.HealthCheck{
		Protocol:               healthcheckConfig["protocol"].(string),
		Port:                   healthcheckConfig["port"].(int),
		CheckIntervalSeconds:   healthcheckConfig["check_interval_seconds"].(int),
		ResponseTimeoutSeconds: healthcheckConfig["response_timeout_seconds"].(int),
		UnhealthyThreshold:     healthcheckConfig["unhealthy_threshold"].(int),
		HealthyThreshold:       healthcheckConfig["healthy_threshold"].(int),
	}

	if v, ok := healthcheckConfig["path"]; ok {
		healthcheck.Path = v.(string)
	}

	return healthcheck
}

func expandForwardingRules(config []interface{}) []godo.ForwardingRule {
	forwardingRules := make([]godo.ForwardingRule, 0, len(config))

	for _, rawRule := range config {
		rule := rawRule.(map[string]interface{})

		r := godo.ForwardingRule{
			EntryPort:      rule["entry_port"].(int),
			EntryProtocol:  rule["entry_protocol"].(string),
			TargetPort:     rule["target_port"].(int),
			TargetProtocol: rule["target_protocol"].(string),
			TlsPassthrough: rule["tls_passthrough"].(bool),
		}

		if v, ok := rule["certificate_id"]; ok {
			r.CertificateID = v.(string)
		}

		forwardingRules = append(forwardingRules, r)

	}

	return forwardingRules
}

func expandDropletIds(configured []interface{}) []int {
	vs := make([]int, 0, len(configured))
	for _, v := range configured {
		val, ok := v.(int)
		if ok && val != 0 {
			vs = append(vs, v.(int))
		}
	}
	return vs
}

func flattenDropletIds(list []int) []interface{} {
	vs := make([]interface{}, 0, len(list))
	for _, v := range list {
		vs = append(vs, v)
	}
	return vs
}

func flattenStickySessions(session *godo.StickySessions) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, 1)

	r := make(map[string]interface{})
	r["type"] = (*session).Type
	r["cookie_name"] = (*session).CookieName
	r["cookie_ttl_seconds"] = (*session).CookieTtlSeconds

	result = append(result, r)

	return result
}

func resourceDigitalOceanLoadbalancerCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*godo.Client)

	log.Printf("[INFO] Create a Loadbalancer Request")
	opts := &godo.LoadBalancerRequest{
		Name:                d.Get("name").(string),
		Region:              d.Get("region").(string),
		Algorithm:           d.Get("algorithm").(string),
		RedirectHttpToHttps: d.Get("redirect_http_to_https").(bool),
		ForwardingRules:     expandForwardingRules(d.Get("forwarding_rule").([]interface{})),
		DropletIDs:          expandDropletIds(d.Get("droplet_ids").(*schema.Set).List()),
	}

	if v, ok := d.GetOk("healthcheck"); ok {
		opts.HealthCheck = expandHealthCheck(v.([]interface{}))
	}

	if v, ok := d.GetOk("sticky_sessions"); ok {
		opts.StickySessions = expandStickySessions(v.([]interface{}))
	}

	log.Printf("[DEBUG] Loadbalancer Create: %#v", opts)
	loadbalancer, _, err := client.LoadBalancers.Create(opts)
	if err != nil {
		return fmt.Errorf("Error creating Loadbalancer: %s", err)
	}

	d.SetId(loadbalancer.ID)

	_, err  := waitForAction()

	return resourceDigitalOceanLoadbalancerRead(d, meta)
}

func resourceDigitalOceanLoadbalancerRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*godo.Client)

	log.Printf("[INFO] Reading the details of the Loadbalancer %s", d.Id())
	loadbalancer, _, err := client.LoadBalancers.Get(d.Id())
	if err != nil {
		return fmt.Errorf("Error retrieving Loadbalancer: %s", err)
	}

	d.Set("name", loadbalancer.Name)
	d.Set("ip", loadbalancer.IP)
	d.Set("algorithm", loadbalancer.Algorithm)
	d.Set("region", loadbalancer.Region.Slug)
	d.Set("redirect_http_to_https", loadbalancer.RedirectHttpToHttps)
	d.Set("droplet_ids", flattenDropletIds(loadbalancer.DropletIDs))
	d.Set("sticky_sessions", flattenStickySessions(loadbalancer.StickySessions))

	return nil

}

func resourceDigitalOceanLoadbalancerUpdate(d *schema.ResourceData, meta interface{}) error {
	//client := meta.(*godo.Client)
	return resourceDigitalOceanLoadbalancerRead(d, meta)
}

func resourceDigitalOceanLoadbalancerDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*godo.Client)

	log.Printf("[INFO] Deleting Loadbalancer: %s", d.Id())
	_, err := client.LoadBalancers.Delete(d.Id())
	if err != nil {
		return fmt.Errorf("Error deleting Loadbalancer: %s", err)
	}

	d.SetId("")
	return nil

}
