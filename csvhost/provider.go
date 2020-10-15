package csvhost

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"csvhost": dataSource(),
		},
		ResourcesMap: map[string]*schema.Resource{},
	}
}
