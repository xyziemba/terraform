package azurerm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
)

// Question to maintainers: what's the right place for this?
func ptrToBool(b bool) *bool {
	newVariable := b
	return &newVariable
}

func resourceArmVirtualMachineExtension() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmVirtualMachineExtensionCreate,
		Read:   resourceArmVirtualMachineExtensionRead,
		Delete: resourceArmVirtualMachineExtensionDelete,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"target_vm_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"extension_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"publisher_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"version": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"public_config": &schema.Schema{
				Type:      schema.TypeString,
				Required:  false,
				Optional:  true,
				ForceNew:  true,
				StateFunc: normalizeJson,
			},

			"private_config": &schema.Schema{
				Type:      schema.TypeString,
				Required:  false,
				Optional:  true,
				ForceNew:  true,
				StateFunc: normalizeJson,
			},
		},
	}
}

func resourceArmVirtualMachineExtensionCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	extensionClient := client.vmExtensionClient

	id, err := parseAzureResourceID(d.Get("target_vm_id").(string))
	if err != nil {
		return err
	}

	resGroup := id.ResourceGroup
	vmName := id.Path["virtualMachines"]

	vmInfo, err := meta.(*ArmClient).vmClient.Get(resGroup, vmName, "")
	if err != nil {
		return err
	}

	name := d.Get("name").(string)
	extensionType := d.Get("extension_name").(string)
	extensionVersion := d.Get("version").(string)
	publisher := d.Get("publisher_name").(string)
	location := vmInfo.Location

	var publicConfiguration map[string]interface{}
	var privateConfiguration map[string]interface{}

	if public_config := d.Get("public_config").(string); public_config != "" {
		if err = json.Unmarshal([]byte(public_config), &publicConfiguration); err != nil {
			return err
		}
	}

	if private_config := d.Get("private_config").(string); private_config != "" {
		if err = json.Unmarshal([]byte(private_config), &privateConfiguration); err != nil {
			return err
		}
	}

	extensionParameters := compute.VirtualMachineExtension{
		Name:     &name,
		Location: location,
		Properties: &compute.VirtualMachineExtensionProperties{
			Type:                    &extensionType,
			TypeHandlerVersion:      &extensionVersion,
			Publisher:               &publisher,
			Settings:                &publicConfiguration,
			ProtectedSettings:       &privateConfiguration,
			AutoUpgradeMinorVersion: ptrToBool(false),
		},
	}

	resp, vmeErr := extensionClient.CreateOrUpdate(resGroup, vmName, name, extensionParameters)
	if vmeErr != nil {
		return vmeErr
	}

	d.SetId(*resp.ID)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"Creating"},
		Target:     []string{"Succeeded"},
		Refresh:    virtualMachineExtensionStateRefreshFunc(client, resGroup, vmName, name),
		Timeout:    20 * time.Minute,
		MinTimeout: 30 * time.Second,
	}
	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("Error waiting for Virtual Machine Extension (%s) to become available: %s", name, err)
	}

	return resourceArmVirtualMachineExtensionRead(d, meta)
}

func virtualMachineExtensionStateRefreshFunc(client *ArmClient, resourceGroupName string, vmName string, vmExtensionName string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		// client.vmClient.
		// TODO: stuck here because I need something like
		//https://management.azure.com/subscriptions/26797607-2e2a-4e56-b4d9-53d2356e15f7/providers/Microsoft.Compute/locations/westus/operations/d0f5168a-e0c4-43e8-9c18-fd84a168e799?api-version=2015-06-15
		res, err := client.vmExtensionClient.Get(resourceGroupName, vmName, vmExtensionName, "")
		if err != nil {
			return nil, "", err
		}

		return res, *res.Properties.ProvisioningState, nil
	}
}

func resourceArmVirtualMachineExtensionDelete(d *schema.ResourceData, meta interface{}) error {
	extensionClient := meta.(*ArmClient).vmExtensionClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resGroup := id.ResourceGroup
	vmName := id.Path["virtualMachines"]
	extensionName := id.Path["extensions"]

	_, err = extensionClient.Delete(resGroup, vmName, extensionName)

	return err
}

func resourceArmVirtualMachineExtensionRead(d *schema.ResourceData, meta interface{}) error {
	extensionClient := meta.(*ArmClient).vmExtensionClient

	id, err := parseAzureResourceID(d.Id())
	if err != nil {
		return err
	}

	resGroup := id.ResourceGroup
	vmName := id.Path["virtualMachines"]
	extensionName := id.Path["extensions"]

	resp, err := extensionClient.Get(resGroup, vmName, extensionName, "")
	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}

	if err != nil {
		return fmt.Errorf("Error making read request on Azure Virtual Machine Extension %s on Virtual Machine %s: %s", extensionName, vmName, err)
	}

	if err = d.Set("extension_name", resp.Properties.Type); err != nil {
		return err
	}

	if err = d.Set("publisher_name", resp.Properties.Publisher); err != nil {
		return err
	}

	if err = d.Set("version", resp.Properties.TypeHandlerVersion); err != nil {
		return err
	}

	return nil
}
