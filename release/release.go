package release

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"github.com/swisscom/waypoint-plugin-cloudfoundry/platform"
)

type ReleaseConfig struct {
	Domain   string `hcl:"domain"`
	Hostname string `hcl:"hostname,optional"`
}

type ReleaseManager struct {
	config ReleaseConfig
}

// Config Implement Configurable
func (rm *ReleaseManager) Config() (interface{}, error) {
	return &rm.config, nil
}

// ReleaseFunc Implement Builder
func (rm *ReleaseManager) ReleaseFunc() interface{} {
	// return a function which will be called by Waypoint
	return rm.release
}

// A BuildFunc does not have a strict signature, you can define the parameters
// you need based on the Available parameters that the Waypoint SDK provides.
// Waypoint will automatically inject parameters as specified
// in the signature at run time.
//
// Available input parameters:
// - context.Context
// - *component.Source
// - *component.JobInfo
// - *component.DeploymentConfig
// - *datadir.Project
// - *datadir.App
// - *datadir.Component
// - hclog.Logger
// - terminal.UI
// - *component.LabelSet

// In addition to default input parameters the platform.Deployment from the Deploy step
// can also be injected.
//
// The output parameters for ReleaseFunc must be a Struct which can
// be serialzied to Protocol Buffers binary format and an error.
// This Output Value will be made available for other functions
// as an input parameter.
//
// If an error is returned, Waypoint stops the execution flow and
// returns an error to the user.
func (rm *ReleaseManager) release(ctx context.Context, ui terminal.UI, src *component.Source, deployment *platform.Deployment) (*Release, error) {
	var release Release

	var hostname string
	if rm.config.Hostname != "" {
		hostname = rm.config.Hostname
	} else {
		hostname = src.App
	}

	sg := ui.StepGroup()
	step := sg.Add("Connecting to Cloud Foundry")

	client, err := platform.GetEnvClient()
	if err != nil {
		step.Abort()
		return nil, fmt.Errorf("unable to create Cloud Foundry client: %v", err)
	}

	step.Update(fmt.Sprintf("Connecting to Cloud Foundry at %s", client.CloudControllerURL))
	step.Done()

	orgGuid := deployment.OrganisationGUID
	spaceGuid := deployment.SpaceGUID

	step = sg.Add(fmt.Sprintf("Getting app info for %v", deployment.Name))

	apps, _, err := client.GetApplications(ccv3.Query{
		Key:    ccv3.OrganizationGUIDFilter,
		Values: []string{orgGuid},
	}, ccv3.Query{
		Key:    ccv3.SpaceGUIDFilter,
		Values: []string{spaceGuid},
	}, ccv3.Query{
		Key:    ccv3.NameFilter,
		Values: []string{deployment.Name},
	})
	if err != nil {
		step.Abort()
		return nil, fmt.Errorf("failed to get app info: %v", err)
	}
	if len(apps) == 0 {
		step.Abort()
		return nil, fmt.Errorf("release failed, app not found")
	}
	step.Done()

	if rm.config.Hostname == "" {
		rm.config.Hostname = src.App
	}

	routeUrl := fmt.Sprintf("%v.%v", rm.config.Hostname, rm.config.Domain)
	step = sg.Add(fmt.Sprintf("Binding route %v to deployment", routeUrl))
	domains, _, err := client.GetDomains(ccv3.Query{
		Key:    ccv3.NameFilter,
		Values: []string{rm.config.Domain},
	})
	if err != nil || len(domains) == 0 {
		step.Abort()
		return nil, fmt.Errorf("failed to get specified domain: %v", err)
	}
	domain := domains[0]

	// Check if route exists already
	routes, _, err := client.GetRoutes(ccv3.Query{
		Key:    ccv3.DomainGUIDFilter,
		Values: []string{domain.GUID},
	}, ccv3.Query{
		Key:    ccv3.HostsFilter,
		Values: []string{hostname},
	})
	if err != nil {
		step.Abort()
		return nil, fmt.Errorf("failed checking if route exists already: %v", err)
	}
	var route resources.Route
	if len(routes) == 0 {
		route, _, err = client.CreateRoute(resources.Route{
			DomainGUID: domain.GUID,
			SpaceGUID:  spaceGuid,
			Host:       hostname,
			Destinations: []resources.RouteDestination{{
				App: resources.RouteDestinationApp{
					GUID: deployment.AppGUID,
				},
			}},
		})
		if err != nil {
			return nil, fmt.Errorf("failed creating route: %v", err)
		}
	} else {
		route = routes[0]
	}

	// Also map
	_, err = client.MapRoute(route.GUID, deployment.AppGUID)
	if err != nil {
		step.Abort()
		return nil, fmt.Errorf("failed to map route: %v", err)
	}
	step.Done()
	release.Url = fmt.Sprintf("%v://%v", route.Protocol, route.URL)

	// Unmap all other applications
	for _, destination := range route.Destinations {
		_, err = client.UnmapRoute(route.GUID, destination.GUID)
		if err != nil {
			return nil, fmt.Errorf("failed to unmap route from destination app with GUID %v", destination.App.GUID)
		}
	}

	return &release, nil
}

func (r *Release) URL() string { return r.Url }

var _ component.Release = (*Release)(nil)
