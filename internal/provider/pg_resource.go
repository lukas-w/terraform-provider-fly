package provider

import (
	"context"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/pkg/apiv1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	hreq "github.com/imroc/req/v3"
	"github.com/superfly/flyctl/api"
)

type pgResource struct {
	flyResource
}

func (r pgResource) getPgClient(ctx context.Context, app string, diags *diag.Diagnostics) *hreq.Client {
	machineAPI := r.machineApi(app)

	var machines apiv1.Machines
	_, err := machineAPI.ListMachines(&machines)
	if err != nil {
		diags.AddError("Error in ListMachines", err.Error())
		return nil
	}
	if len(machines) == 0 {
		diags.AddError("Can't connect to postgres", fmt.Sprintf("App %s has no machines", app))
		return nil
	}
	machines = machines.WithoutInactive()
	if len(machines) == 0 {
		diags.AddError("Can't connect to postgres", fmt.Sprintf("App %s has no active machines", app))
		return nil
	}

	leader := findLeader(machines)
	if leader == nil {
		diags.AddError("Can't connect to postgres", fmt.Sprintf("Postgres cluster %s has no leader", app))
		return nil
	}

	return r.httpClient.Clone().SetBaseURL(fmt.Sprintf("http://%s:5500", leader.PrivateIP))
}

func machineRole(machine *api.Machine) (role string) {
	role = "unknown"

	for _, check := range machine.Checks {
		if check.Name == "role" {
			if check.Status == "passing" {
				role = check.Output
			} else {
				role = "error"
			}
			break
		}
	}
	return role
}

func findLeader(machines apiv1.Machines) *api.Machine {
	for _, machine := range machines {
		if machineRole(machine) == "leader" {
			return machine
		}
	}
	return nil
}
