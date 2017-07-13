package esscsvhost

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func dataSource() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRead,

		Schema: map[string]*schema.Schema{
			"csvfile": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},

			"query": &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"result": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"hostname": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"address": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"gateway": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"subnet": &schema.Schema{
							Type:     schema.TypeInt,
							Computed: true,
						},
						"cpu": &schema.Schema{
							Type:     schema.TypeInt,
							Computed: true,
						},
						"memory": &schema.Schema{
							Type:     schema.TypeInt,
							Computed: true,
						},
						"vapp": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"network": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"template": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceRead(d *schema.ResourceData, meta interface{}) error {
	csvfile := d.Get("csvfile").(string)
	query := d.Get("query").(map[string]interface{})
	data, err := ioutil.ReadFile(csvfile)
	reader := csv.NewReader(strings.NewReader(string(data)))
	columns := []string{"hostname", "address", "gateway", "subnet", "cpu", "memory", "vapp", "network", "template"}

	rows := make([]map[string]interface{}, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read CSV file %q: %s", csvfile, err)
		}
		// skip the header row if provided
		var header = true
		for i, v := range record {
			if v != columns[i] {
				header = false
			}
		}
		if header {
			continue
		}
		row := make(map[string]interface{})
		for i, k := range columns {
			row[k], err = strconv.Atoi(record[i])
			if err != nil {
				row[k] = string(record[i])
			}
			log.Println(row)
		}
		rows = append(rows, row)
	}
	resultJson, err := json.MarshalIndent(&rows, "", "    ")
	check(err)

	result := make([]map[string]interface{}, 0)
	err = json.Unmarshal(resultJson, &result)
	if err != nil {
		return fmt.Errorf("command %q produced invalid JSON: %s", csvfile, err)
	}

	// poor mans filter to JSON array
	filtered := make([]map[string]interface{}, 0)
	if query != nil {
		for _, item := range result {
			var add = true
			for q, v := range query {
				log.Println(item[q])
				log.Println(v)
				endsWith := strings.HasSuffix(item[q].(string), v.(string))
				if item[q] != v && !endsWith {
					add = false
				}
			}
			if add {
				filtered = append(filtered, item)
			}
		}
	}

	log.Println("=============>>>>>>>>>>>>>>>>>")
	for i := range filtered {
		log.Println(filtered[i])
	}
	log.Println("<<<<<<<<<<<<<=================")

	d.Set("result", &filtered)
	d.SetId("-")
	return nil
}
