package csvhost

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/hcl"
	"gopkg.in/resty.v1"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

var connection *resty.Client
var datastores []string
var credentials map[string]string
var server string

var connectOnce sync.Once
var dsOnce sync.Once

var tfvarsOnce sync.Once
var tfdomainOnce sync.Once

func getCredentials() map[string]string {
	tfvarsOnce.Do(func() {
		credentials = make(map[string]string, 2)
		data, err := ioutil.ReadFile("terraform.tfvars")
		if err != nil {
			panic(fmt.Sprintf("failed to read terraform.tfvars file - %v", err.Error()))
		}

		tfvars := make(map[string]interface{}, 0)
		err = hcl.Unmarshal(data, &tfvars)
		if err != nil {
			panic(fmt.Sprintf("failed to parse terraform.tfvars file", err.Error()))
		}
		credentials["user"] = tfvars["vsphere_user"].(string)
		credentials["pass"] = tfvars["vsphere_password"].(string)
	})

	return credentials
}

func getDomain() string {
	tfdomainOnce.Do(func() {
		data, err := ioutil.ReadFile("variables.tf")
		if err != nil {
			panic(fmt.Sprintf("failed to read variables.tf file - %v", err.Error()))
		}

		tfvars := make(map[string]interface{}, 0)
		err = hcl.Unmarshal(data, &tfvars)
		if err != nil {
			panic(fmt.Sprintf("failed to parse variables.tf file", err.Error()))
		}
		variable := tfvars["variable"].([]map[string]interface{})
		vserver := variable[0]["vsphere_server"].([]map[string]interface{})
		server = vserver[0]["default"].(string)
	})
	return server
}

func connect(domain string) *resty.Client {
	credentials := getCredentials()
	connectOnce.Do(func() {
		connection = resty.New()
		connection.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		connection.SetBasicAuth(credentials["user"], credentials["pass"])
		connection.RemoveProxy()
		resp, err := connection.R().Post(fmt.Sprintf("https://%v/rest/com/vmware/cis/session", domain))
		if err != nil {
			panic("failed to connect: " + err.Error())
		}
		data := make(map[string]interface{}, 0)
		if err := json.Unmarshal(resp.Body(), &data); err != nil {
			panic(err.Error())
		}

		connection.SetCookie(&http.Cookie{
			Name:     "vmware-api-session-id",
			Value:    data["value"].(string),
			Path:     "/",
			Domain:   domain,
			MaxAge:   36000,
			HttpOnly: false,
			Secure:   true,
		})
	})
	return connection
}

func query(what string) map[string]interface{} {
	var domain = getDomain()
	var url = fmt.Sprintf("https://%v/rest/vcenter/%v", domain, what)
	resp, err := connect(domain).R().
		SetHeader("Accept", "application/json").
		Get(url)
	if err != nil {
		panic("failed to connect: " + err.Error())
	}
	// Unmarshal the VM to an interface
	data := make(map[string]interface{}, 0)
	if err = json.Unmarshal(resp.Body(), &data); err != nil {
		panic(err.Error())
	}
	return data
}

func getClusterPrefix() string {
	return "Odd" // should come from config h/c for now
}

func getDatastores(prefix string) []string {
	dsOnce.Do(func() {
		var ds = query("datastore")["value"].([]interface{})
		datastores = make([]string, 0)
		//var prefix = getClusterPrefix()
		for _, value := range ds {
			name := value.(map[string]interface{})["name"].(string)
			if strings.HasPrefix(name, prefix) {
				datastores = append(datastores, name)
			}
		}
	})
	return datastores
}

func getDisks(data map[string]interface{}) []string {
	var value = data["value"].(map[string]interface{})
	var disks = value["disks"].([]interface{})

	var vmdks = make([]string, len(disks))
	for _, v := range disks {
		var dvalue = v.(map[string]interface{})["value"].(map[string]interface{})
		var label = strings.Split(dvalue["label"].(string), " ")
		index, err := strconv.Atoi(label[len(label)-1])
		if err != nil {
			panic("Hard disk label doesn't end in an integer")
		}
		var backing = dvalue["backing"].(map[string]interface{})
		vmdks[(index - 1)] = backing["vmdk_file"].(string)
	}
	return vmdks
}

func getLun(vmdk string) string {
	return strings.TrimLeft(strings.TrimRight(vmdk, "]"), "[")
}

func getImage(vmdk string) string {
	return strings.Split(strings.Split(vmdk, "/")[1], ".")[0]
}

func getVm(vmname string) (string, error) {
	vmids := query(fmt.Sprintf("vm?filter.names.1=%v", vmname))["value"].([]interface{})
	if 0 == len(vmids) {
		// we would be able to continue here as we're probably creating...
		fmt.Printf("[ERROR] - No VM found with name %v\n", vmname)
		return "", nil
	} else if 1 != len(vmids) {
		return "", fmt.Errorf("Multiple VMs found with name %v", vmname)
	}
	return vmids[0].(map[string]interface{})["vm"].(string), nil
}

func getVmDetails(vmid string) map[string]interface{} {
	return query(fmt.Sprintf("vm/%v", vmid))
}

func getVmList() []string {
	data := query("vm")["value"].([]interface{})
	vmlist := make([]string, len(data))
	for i, value := range data {
		vmlist[i] = value.(map[string]interface{})["name"].(string)
	}
	return vmlist
}

func randomDS(clusterPrefix string) string {
	datastores := getDatastores(clusterPrefix)
	return datastores[rand.Intn(len(datastores))]
}
