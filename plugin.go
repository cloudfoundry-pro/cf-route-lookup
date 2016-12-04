package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv2"
	"code.cloudfoundry.org/cli/plugin"
)

type BasicPlugin struct{}

type domainsResponse struct {
	NextUrl   string        `json:"next_url"`
	Resources []ccv2.Domain `json:"resources"`
}

type routesResponse struct {
	NextUrl   string       `json:"next_url"`
	Resources []ccv2.Route `json:"resources"`
}

// possibleDomains returns all domain levels, down to the second-level domain (SLD), in order.
func getPossibleDomains(hostname string) []string {
	parts := strings.Split(hostname, ".")
	numCombinations := len(parts) - 1
	possibleDomains := make([]string, numCombinations)
	for i := 0; i < numCombinations; i++ {
		possibleDomains[i] = strings.Join(parts[i:], ".")
	}
	return possibleDomains
}

func apiCall(cliConnection plugin.CliConnection, path string) (body string, err error) {
	// based on https://github.com/krujos/cfcurl/blob/320854091a119f220102ba356e507c361562b221/cfcurl.go
	bodyLines, err := cliConnection.CliCommandWithoutTerminalOutput("curl", path)
	if err != nil {
		return
	}
	body = strings.Join(bodyLines, "\n")
	return
}

func getDomains(cliConnection plugin.CliConnection, names []string) (domains []ccv2.Domain, err error) {
	// based on https://github.com/ECSTeam/buildpack-usage/blob/e2f7845f96c021fa7f59d750adfa2f02809e2839/command/buildpack_usage_cmd.go#L161-L167

	domains = make([]ccv2.Domain, 0)

	endpoints := [...]string{"/v2/private_domains", "/v2/shared_domains"}

	params := url.Values{}
	params.Set("q", "name IN "+strings.Join(names, ","))
	params.Set("results-per-page", "100")
	queryString := params.Encode()

	for _, endpoint := range endpoints {
		url := endpoint + "?" + queryString
		fmt.Println(url)

		// paginate
		for url != "" {
			var body string
			body, err = apiCall(cliConnection, url)
			if err != nil {
				return
			}

			var data domainsResponse
			err = json.Unmarshal([]byte(body), &data)
			if err != nil {
				return
			}

			domains = append(domains, data.Resources...)
			url = data.NextUrl
		}
	}

	return
}

func getDomain(cliConnection plugin.CliConnection, hostname string) (matchingDomain ccv2.Domain, found bool, err error) {
	possibleDomains := getPossibleDomains(hostname)
	fmt.Printf("%#v\n", possibleDomains)

	domains, err := getDomains(cliConnection, possibleDomains)
	if err != nil {
		return
	}
	fmt.Println("Matching domains:", domains)

	for _, possibleDomain := range possibleDomains {
		for _, domain := range domains {
			if domain.Name == possibleDomain {
				found = true
				matchingDomain = domain
				return
			}
		}
	}

	return
}

func getRoutes(cliConnection plugin.CliConnection) (routes []ccv2.Route, err error) {
	// based on https://github.com/ECSTeam/buildpack-usage/blob/e2f7845f96c021fa7f59d750adfa2f02809e2839/command/buildpack_usage_cmd.go#L161-L167

	routes = make([]ccv2.Route, 0)
	url := "/v2/routes?results-per-page=100"

	// paginate
	for url != "" {
		var body string
		body, err = apiCall(cliConnection, url)
		if err != nil {
			return
		}

		var data routesResponse
		err = json.Unmarshal([]byte(body), &data)
		if err != nil {
			return
		}

		routes = append(routes, data.Resources...)
		url = data.NextUrl
	}

	return
}

func (c *BasicPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	if args[0] == "basic-plugin-command" {
		fmt.Println("Running the basic-plugin-command")

		// TODO check for argument length

		hostname := args[1]
		domain, domainFound, err := getDomain(cliConnection, hostname)
		if err != nil {
			log.Fatal("Error retrieving the domains.")
		}
		if !domainFound {
			log.Fatal("Could not find matching domain.")
		}
		if domain.Name == hostname {
			fmt.Println("It's a domain! GUID:", domain.GUID)
			return
		}

		routes, err := getRoutes(cliConnection)
		if err != nil {
			log.Fatal("Error retrieving the routes.")
		}
		fmt.Println(len(routes), "routes found.")

		subdomain := strings.Split(hostname, ".")[0]
		matches := make([]ccv2.Route, 0, len(routes))
		for _, route := range routes {
			// TODO handle private domains, which may not have a Host
			if route.Host == subdomain {
				fmt.Println("Subdomain match!", subdomain)
				matches = append(matches, route)
			}
		}
		if len(matches) == 0 {
			fmt.Println("Domain not found.")
		}
	}
}

func (c *BasicPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "MyBasicPlugin",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 0,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "basic-plugin-command",
				HelpText: "Basic plugin command's help text",
				UsageDetails: plugin.Usage{
					Usage: "basic-plugin-command\n   cf basic-plugin-command",
				},
			},
		},
	}
}

func main() {
	plugin.Start(new(BasicPlugin))
}
