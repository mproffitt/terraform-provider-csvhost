package main

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/mproffitt/terraform-provider-csvhost/csvhost"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: csvhost.Provider})
}
