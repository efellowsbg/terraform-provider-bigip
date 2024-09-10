package bigip

import (
	"context"
	"fmt"
	"log"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceBigIPAPMWebtop() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceBigIPAPMWebtopRead,
		CreateContext: resourceBigIPAPMWebtopCreate,
		UpdateContext: resourceBigIPAPMWebtopUpdate,
		DeleteContext: resourceBigIPAPMWebtopDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the webtop. This field is not updatable.",
			},
			"tm_partition": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The tmPartition of the webtop. This field is not updatable.",
			},
			"partition": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The partition of the webtop. This field is not updatable.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The description of the webtop. This field is updatable.",
			},
			"customization_group": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The customization group of the webtop. This field is updatable.",
			},
			"initial_state": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{string(bigip.InitialStateCollapsed), string(bigip.InitialStateExpanded)}, false),
				Description:  "The initial state of the webtop. This field is updatable.",
			},
			"customization_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{string(bigip.CustomizationTypeModern), string(bigip.CustomizationTypeStandard)}, false),
				Description:  "The customization type of the webtop. This field is updatable.",
			},
			"link_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{string(bigip.LinkTypeUri)}, false),
				Description:  "The link type of the webtop. This field is updatable.",
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{string(bigip.WebtopTypePortal), string(bigip.WebtopTypeFull), string(bigip.WebtopTypeNetwork)}, false),
				Description:  "The type of the webtop. This field is updatable.",
			},
			"show_search": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to show search in the webtop. This field is updatable.",
			},
			"warning_on_close": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to show a warning on close. This field is updatable.",
			},
			"url_entry_field": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to show the URL entry field. This field is updatable.",
			},
			"resource_search": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to enable resource search. This field is updatable.",
			},
			"minimize_to_tray": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether to minimize to tray. This field is updatable.",
			},
			"location_specific": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Whether the webtop is location specific. This field is updatable.",
			},
			"full_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The full path of the webtop. This field is computed and not updatable.",
			},
			"self_link": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The self link of the webtop. This field is computed and not updatable.",
			},
			"customization_group_reference": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The customization group reference of the webtop. This field is computed and not updatable.",
			},
			"generation": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The generation of the webtop. This field is computed and not updatable.",
			},
		},
	}
}

func resourceBigIPAPMWebtopRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	name := d.Get("name").(string)
	webtop, err := client.GetWebtop(ctx, name)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading webtop %w", err))
	}
	if webtop == nil {
		log.Printf("[DEBUG] webtop (%s) not found, removing from state", name)
		d.SetId("")
		return nil
	}

	d.SetId(webtop.Name)
	return nil
}

func resourceBigIPAPMWebtopCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	webtop := &bigip.Webtop{
		Name:        d.Get("name").(string),
		TMPartition: d.Get("tm_partition").(string),
		Partition:   d.Get("partition").(string),
		WebtopConfig: bigip.WebtopConfig{
			Description:        d.Get("description").(string),
			CustomizationGroup: d.Get("customization_group").(string),
			InitialState:       d.Get("initial_state").(bigip.InitialState),
			CustomizationType:  d.Get("customization_type").(bigip.CustomizationType),
			LinkType:           d.Get("link_type").(bigip.LinkType),
			Type:               d.Get("type").(bigip.WebtopType),
			ShowSearch:         d.Get("show_search").(bigip.BooledString),
			WarningOnClose:     d.Get("warning_on_close").(bigip.BooledString),
			UrlEntryField:      d.Get("url_entry_field").(bigip.BooledString),
			ResourceSearch:     d.Get("resource_search").(bigip.BooledString),
			MinimizeToTray:     d.Get("minimize_to_tray").(bigip.BooledString),
			LocationSpecific:   d.Get("location_specific").(bigip.BooledString),
		},
	}
	err := client.CreateWebtop(ctx, *webtop)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating webtop %w", err))
	}
	d.SetId(webtop.Name)
	return nil
}

func resourceBigIPAPMWebtopUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	webtop := &bigip.Webtop{
		WebtopConfig: bigip.WebtopConfig{
			Description:        d.Get("description").(string),
			CustomizationGroup: d.Get("customization_group").(string),
			InitialState:       d.Get("initial_state").(bigip.InitialState),
			CustomizationType:  d.Get("customization_type").(bigip.CustomizationType),
			LinkType:           d.Get("link_type").(bigip.LinkType),
			Type:               d.Get("type").(bigip.WebtopType),
			ShowSearch:         d.Get("show_search").(bigip.BooledString),
			WarningOnClose:     d.Get("warning_on_close").(bigip.BooledString),
			UrlEntryField:      d.Get("url_entry_field").(bigip.BooledString),
			ResourceSearch:     d.Get("resource_search").(bigip.BooledString),
			MinimizeToTray:     d.Get("minimize_to_tray").(bigip.BooledString),
			LocationSpecific:   d.Get("location_specific").(bigip.BooledString),
		},
	}
	err := client.ModifyWebtop(ctx, webtop.Name, webtop.WebtopConfig)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error updating webtop %w", err))
	}

	return nil
}

func resourceBigIPAPMWebtopDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	err := client.DeleteWebtop(ctx, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting webtop %w", err))
	}
	d.SetId("")
	return nil
}
