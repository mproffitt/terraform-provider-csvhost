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
					},
				},
			},
		},
	}
}

func dataSourceRead(d *schema.ResourceData, meta interface{}) error {

	csvfile := d.Get("csvfile").(string)
	//query := d.Get("query").(map[string]interface{})
	data, err := ioutil.ReadFile(csvfile)
	reader := csv.NewReader(strings.NewReader(string(data)))
	columns := []string{"hostname", "address", "gateway", "subnet", "cpu", "memory", "vapp", "network"}

	rows := make([]map[string]interface{}, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read CSV file %q: %s", csvfile, err)
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
	err = ioutil.WriteFile("bob.json", resultJson, 0644)
	check(err)

	result := make([]map[string]interface{}, 0)
	err = json.Unmarshal(resultJson, &result)
	if err != nil {
		return fmt.Errorf("command %q produced invalid JSON: %s", csvfile, err)
	}
	log.Println("======================= %#v =======================", csvfile)
	log.Println("=============>>>>>>>>>>>>>>>>>")
	for i := range result {
		log.Println(result[i])
	}
	log.Println("<<<<<<<<<<<<<=================")
	log.Println("===================================================")

	d.Set("result", &result)
	d.SetId("-")
	return nil
}
