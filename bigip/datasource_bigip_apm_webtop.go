package bigip

import (
	"context"
	"log"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceBigIPAPMWebtop() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceBigIPAPMWebtopRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the APM Webtop",
			},
			"full_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The full path of the APM Webtop",
			},
			"generation": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The generation of the APM Webtop",
			},
			"self_link": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The self link of the APM Webtop",
			},
			"customization_group": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The customization group of the APM Webtop",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The description of the APM Webtop",
			},
			"fallback_section_initial_state": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The fallback section initial state of the APM Webtop",
			},
			"link_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The link type of the APM Webtop",
			},
			"location_specific": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the APM Webtop is location specific",
			},
			"minimize_to_tray": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the APM Webtop minimizes to tray",
			},
			"show_search": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the APM Webtop shows search",
			},
			"show_url_entry_field": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the APM Webtop shows URL entry field",
			},
			"warn_when_closed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the APM Webtop warns when closed",
			},
			"webtop_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The type of the APM Webtop",
			},
		},
	}
}

func dataSourceBigIPAPMWebtopRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	log.Printf("[INFO] Retrieving APM Webtop %s", d.Get("name").(string))
	webtop, err := client.GetWebtop(ctx, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}
	diag := setAPMWebtopResourceData(d, webtop)
	if diag != nil {
		d.SetId("")
		return diag
	}
	log.Println("[INFO] Retrieved APM Webtop")
	d.SetId(webtop.FullPath)
	return nil
}

func setAPMWebtopResourceData(d *schema.ResourceData, webtop bigip.WebtopRead) diag.Diagnostics {
	err := d.Set("name", webtop.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("full_path", webtop.FullPath)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("generation", webtop.Generation)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("self_link", webtop.SelfLink)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("customization_group", webtop.CustomizationGroup)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("description", webtop.Description)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("fallback_section_initial_state", webtop.InitialState)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("link_type", webtop.LinkType)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("location_specific", webtop.LocationSpecific)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("minimize_to_tray", webtop.MinimizeToTray)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("show_search", webtop.ShowSearch)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("show_url_entry_field", webtop.UrlEntryField)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("warn_when_closed", webtop.WarningOnClose)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("webtop_type", webtop.Type)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
