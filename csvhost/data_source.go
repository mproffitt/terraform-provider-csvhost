package csvhost

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
	"time"
)

var MAX_DISKS = 4

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

			"clusterPrefix": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
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
						"expires": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"power": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk1": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk1lun": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk2": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk2lun": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk3": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk3lun": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk4": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk4lun": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk5": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"disk5lun": &schema.Schema{
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
	clusterPrefix := d.Get("clusterPrefix").(string)

	data, err := ioutil.ReadFile(csvfile)
	reader := csv.NewReader(strings.NewReader(string(data)))
	columns := []string{"hostname", "address", "gateway", "subnet", "cpu", "memory", "vapp", "network", "template", "expires"}

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
	log.Println("beginning filter search....")
	if query != nil {
		for _, item := range result {
			var add = true
			for q, v := range query {
				log.Printf("item[%v] == %v\n", item[q].(string), v.(string))
				endsWith := strings.HasSuffix(item[q].(string), v.(string))
				if item[q] != v && !endsWith {
					add = false
				}
			}

			if item["expires"] == "" || item["expires"] == nil {
				// set date to a year from now...
				y, m, d := time.Now().Date()
				item["expires"] = time.Date(y+1, m, d, 0, 0, 0, 0, time.Now().Location()).Format("2006-01-02")
			}
			item["power"] = "ignored" // default to ignored - we don't care about existing state as this could interfere
			// with maintenance of existing machines.
			date, err := time.Parse("2006-01-02", item["expires"].(string))
			if err != nil {
				// try formatting the date in dd/mm/YYYY format
				// and yes, this is because of M$ excel f***ing with the format
				date, err = time.Parse("02/01/2006", item["expires"].(string))
				if err != nil {
					return fmt.Errorf("Invalid date format for expires. Format should be 'YYYY-MM-DD'")
				}
			}
			year, month, day := time.Now().Date()
			delta := time.Date(year, month, day, 0, 0, 0, 0, time.Now().Location()).Sub(date).Hours()
			if delta > 0 {
				item["power"] = "poweredOff"
			}

			// don't add any machines to the list that are > 7 days past expiry
			// these should already have been moved in the state file by python.
			if delta >= 168 { // 7 * 24 = 7 days
				add = false
			}

			if add {
				log.Printf("============= RETRIEVING DISKS FOR %v >>>>>>>>>>>>>>>>>\n", item["hostname"].(string))
				lun := randomDS(clusterPrefix)
				vmid, err := getVm(item["hostname"].(string))
				if err != nil {
					panic(err.Error())
				}

				if vmid != "" {
					details := getVmDetails(vmid)
					disks := getDisks(details)
					for index, value := range disks {
						if index >= MAX_DISKS {
							break
						}
						item[fmt.Sprintf("disk%v", (index+1))] = getImage(strings.Split(value, " ")[1])
						item[fmt.Sprintf("disk%vlun", (index+1))] = getLun(strings.Split(value, " ")[0])
					}
					log.Printf("Found %d disks\n", len(disks))
				} else {
					for i := 0; i <= MAX_DISKS; i++ {
						diskName := fmt.Sprintf("%v_%d", item["hostname"], (i + 1))
						if i == 0 {
							diskName = item["hostname"].(string)
						}
						item[fmt.Sprintf("disk%v", (i+1))] = diskName
						item[fmt.Sprintf("disk%vlun", (i+1))] = lun
					}
				}

				filtered = append(filtered, item)
				log.Printf(
					"============= (%d hosts for vapp %v - size %v) >>>>>>>>>>>>>>>>>\n",
					len(filtered),
					item["vapp"],
					item["template"])
			}
		}
	}

	log.Println("============= FILTERED >>>>>>>>>>>>>>>>>")
	for i := range filtered {
		log.Println(filtered[i])
	}
	log.Println("<<<<<<<<<<<<<=================")

	d.Set("result", &filtered)
	d.SetId("-")
	return nil
}
