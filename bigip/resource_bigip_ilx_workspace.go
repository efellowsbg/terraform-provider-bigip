package bigip

import (
	"context"
	"fmt"
	"log"

	bigip "github.com/f5devcentral/go-bigip"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBigIPILXWorkspace() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceBigIPILXWorkspaceRead,
		CreateContext: resourceBigIPILXWorkspaceCreate,
		UpdateContext: resourceBigIPILXWorkspaceUpdate,
		DeleteContext: resourceBigIPILXWorkspaceDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceBigIPILXWorkspaceRead(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	name := d.Get("name").(string)
	workspace, err := client.GetWorkspace(ctx, name)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading workspace %w", err))
	}
	if workspace == nil {
		log.Printf("[DEBUG] workspace (%s) not found, removing from state", name)
		d.SetId("")
		return nil
	}

	d.SetId(workspace.Name)
	return nil
}

func resourceBigIPILXWorkspaceCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	err := client.CreateWorkspace(ctx, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating workspace %w", err))
	}
	d.SetId(d.Get("name").(string))
	return nil
}

func resourceBigIPILXWorkspaceUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	err := client.PatchWorkspace(ctx, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error updating workspace %w", err))
	}

	return nil
}

func resourceBigIPILXWorkspaceDelete(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	client := m.(*bigip.BigIP)
	err := client.DeleteWorkspace(ctx, d.Get("name").(string))
	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting workspace %w", err))
	}
	d.SetId("")
	return nil
}
