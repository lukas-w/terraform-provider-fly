package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func (mr flyMachineResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			StateUpgrader: machineUpgrade0,
		},
	}
}

func machineUpgrade0(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("app"), data.App)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("config"), data.Config)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("wait"), data.Wait)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("wait_for_checks"), data.WaitForChecks)...)

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), data.Id)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("name"), data.Name)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("region"), data.Region)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("private_ip"), data.PrivateIP)...)

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("env"), data.Config.Env)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("processes"), data.Config.Processes)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("image"), data.Config.Image)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metadata"), data.Config.Metadata)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("mounts"), data.Config.Mounts)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("restart"), data.Config.Restart)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("services"), data.Config.Services)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("guest"), data.Config.Guest)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("metrics"), data.Config.Metrics)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("checks"), data.Config.Checks)...)

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("exec"), data.Config.Init.Exec)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("entrypoint"), data.Config.Init.Entrypoint)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("cmd"), data.Config.Init.Cmd)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
