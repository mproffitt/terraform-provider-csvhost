package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/mproffitt/terraform-provider-esscsvhost/esscsvhost"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: esscsvhost.Provider})
}
