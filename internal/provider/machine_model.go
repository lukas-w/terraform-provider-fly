package provider

import (
	"context"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/superfly/flyctl/api"
)

type TfPort struct {
	Port       types.Int64    `tfsdk:"port"`
	Handlers   []types.String `tfsdk:"handlers"`
	ForceHttps types.Bool     `tfsdk:"force_https"`
}

type TfServiceConcurrency struct {
	Type      types.String `tfsdk:"type"`
	HardLimit types.Int64  `tfsdk:"hard_limit"`
	SoftLimit types.Int64  `tfsdk:"soft_limit"`
}

type TfService struct {
	Ports        []TfPort              `tfsdk:"ports"`
	Protocol     types.String          `tfsdk:"protocol"`
	InternalPort types.Int64           `tfsdk:"internal_port"`
	Concurrency  *TfServiceConcurrency `tfsdk:"concurrency"`
}

type TfGuest struct {
	Cpus       types.Int64  `tfsdk:"cpus"`
	MemoryMb   types.Int64  `tfsdk:"memory_mb"`
	CpuKind    types.String `tfsdk:"cpu_kind"`
	KernelArgs []string     `tfsdk:"kernel_args"`
}

var TfGuestAttrTypes = map[string]attr.Type{
	"cpus":        types.Int64Type,
	"memory_mb":   types.Int64Type,
	"cpu_kind":    types.StringType,
	"kernel_args": types.ListType{ElemType: types.StringType},
}

type TfProcess struct {
	Exec       []string     `tfsdk:"exec"`
	Entrypoint []string     `tfsdk:"entrypoint"`
	Cmd        []string     `tfsdk:"cmd"`
	User       types.String `tfsdk:"user"`
	Env        types.Map    `tfsdk:"env"`
}

type TfRestart struct {
	Policy     types.String `tfsdk:"policy"`
	MaxRetries types.Int64  `tfsdk:"max_retries"`
}

type TfMetric struct {
	Port types.Int64  `tfsdk:"port"`
	Path types.String `tfsdk:"path"`
}

type TfCheck struct {
	Type       types.String      `tfsdk:"type"`
	Port       types.Int64       `tfsdk:"port"`
	Interval   DurationAttrValue `tfsdk:"interval"`
	Timeout    DurationAttrValue `tfsdk:"timeout"`
	HttpMethod types.String      `tfsdk:"http_method"`
	HttpPath   types.String      `tfsdk:"http_path"`
}

type TfMachineMount struct {
	//Encrypted types.Bool   `tfsdk:"encrypted"`
	Path types.String `tfsdk:"path"`
	//SizeGb    types.Int64  `tfsdk:"size_gb"`
	Volume types.String `tfsdk:"volume"`
}

type TfMachineInit struct {
	Exec       []string `tfsdk:"exec"`
	Entrypoint []string `tfsdk:"entrypoint"`
	Cmd        []string `tfsdk:"cmd"`
}

type flyMachineConfigData struct {
	Env       types.Map          `tfsdk:"env"`
	Init      *TfMachineInit     `tfsdk:"init"`
	Processes []TfProcess        `tfsdk:"processes"`
	Image     types.String       `tfsdk:"image"`
	Metadata  types.Map          `tfsdk:"metadata"`
	Mounts    []TfMachineMount   `tfsdk:"mounts"`
	Restart   *TfRestart         `tfsdk:"restart"`
	Services  []TfService        `tfsdk:"services"`
	Guest     types.Object       `tfsdk:"guest"`
	Metrics   *TfMetric          `tfsdk:"metrics"`
	Schedule  *string            `tfsdk:"schedule"`
	Checks    map[string]TfCheck `tfsdk:"checks"`
}

type flyMachineResourceData struct {
	App           types.String         `tfsdk:"app"`
	Config        flyMachineConfigData `tfsdk:"config"`
	Wait          types.Bool           `tfsdk:"wait"`
	WaitForChecks types.Bool           `tfsdk:"wait_for_checks"`

	Id        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Region    types.String `tfsdk:"region"`
	PrivateIP types.String `tfsdk:"private_ip"`
}

func (c *TfCheck) updateFromApi(ac api.MachineCheck, _ *diag.Diagnostics) {
	c.Type = types.String{Value: ac.Type}
	c.Port = types.Int64{Value: int64(ac.Port)}
	if ac.Interval == nil {
		c.Interval = DurationNull()
	} else if c.Interval.IsUnknown() || c.Interval.IsNull() || !c.Interval.EqualDuration(ac.Interval.Duration) {
		c.Interval = DurationValueFromDuration(ac.Interval.Duration)
	}
	if ac.Timeout == nil {
		c.Timeout = DurationNull()
	} else if c.Timeout.IsUnknown() || c.Timeout.IsNull() || !c.Timeout.EqualDuration(ac.Timeout.Duration) {
		c.Timeout = DurationValueFromDuration(ac.Timeout.Duration)
	}
	if ac.HTTPPath != nil {
		c.HttpPath.Value = *ac.HTTPPath
	} else {
		c.HttpPath.Null = true
	}
	if ac.HTTPMethod != nil {
		c.HttpMethod.Value = *ac.HTTPMethod
	} else {
		c.HttpMethod.Null = true
	}
}

func (t *TfMachineInit) toApi() api.MachineInit {
	if t == nil {
		return api.MachineInit{}
	}
	return api.MachineInit{
		Exec:       t.Exec,
		Entrypoint: t.Entrypoint,
		Cmd:        t.Cmd,
	}
}

func (d *flyMachineConfigData) updateGuestFromApiData(g *api.MachineGuest) {
	if g != nil {
		d.Guest = types.Object{AttrTypes: TfGuestAttrTypes, Attrs: map[string]attr.Value{
			"cpus":        types.Int64{Value: int64(g.CPUs)},
			"memory_mb":   types.Int64{Value: int64(g.MemoryMB)},
			"cpu_kind":    types.String{Value: g.CPUKind},
			"kernel_args": utils.StringSliceToTfList(g.KernelArgs),
		}}
	} else {
		d.Guest = types.Object{AttrTypes: TfGuestAttrTypes, Null: true}
	}
}

func (d *flyMachineConfigData) updateFromApiCreateData(c api.MachineConfig) (diags diag.Diagnostics) {
	d.Metadata = utils.KVToTfMap(c.Metadata, types.StringType, true)
	d.Env = utils.KVToTfMap(c.Env, types.StringType, true)
	d.updateGuestFromApiData(c.Guest)
	if len(c.Mounts) > 0 {
		var mounts []TfMachineMount
		for _, m := range c.Mounts {
			mounts = append(mounts, TfMachineMount{
				//Encrypted: types.Bool{Value: m.Encrypted},
				Path: types.String{Value: m.Path},
				//SizeGb:    types.Int64{Value: int64(m.SizeGb)},
				Volume: types.String{Value: m.Volume},
			})
		}
		d.Mounts = mounts
	} else {
		d.Mounts = nil
	}
	return
}

func (d *flyMachineConfigData) updateFromApiData(c api.MachineConfig) (diags diag.Diagnostics) {
	diags.Append(d.updateFromApiCreateData(c)...)

	d.Image = types.String{Value: c.Image}
	if len(c.Init.Exec) > 0 || len(c.Init.Cmd) > 0 || len(c.Init.Entrypoint) > 0 {
		d.Init = &TfMachineInit{
			Cmd:        c.Init.Cmd,
			Entrypoint: c.Init.Entrypoint,
			Exec:       c.Init.Exec,
		}
	} else {
		d.Init = nil
	}

	tfServices := ServicesToTfServices(c.Services)
	if d.Services == nil && len(tfServices) == 0 {
		tfServices = nil
	}
	d.Services = tfServices

	if len(c.Processes) > 0 {
		d.Processes = make([]TfProcess, len(c.Processes))
		for i, p := range c.Processes {
			d.Processes[i] = TfProcess{
				Exec:       p.ExecOverride,
				Entrypoint: p.EntrypointOverride,
				Cmd:        p.CmdOverride,
				User:       types.String{Value: p.UserOverride},
				Env:        utils.KVToTfMap(p.ExtraEnv, types.StringType, true),
			}
		}
	} else {
		d.Processes = nil
	}

	if c.Metrics != nil {
		d.Metrics = &TfMetric{
			Port: types.Int64{Value: int64(c.Metrics.Port)},
			Path: types.String{Value: c.Metrics.Path},
		}
	}

	if c.Schedule != "" {
		d.Schedule = &c.Schedule
	}

	if len(c.Checks) > 0 {
		if d.Checks == nil {
			d.Checks = make(map[string]TfCheck)
		}
		for k, c := range c.Checks {
			check, _ := d.Checks[k]
			check.updateFromApi(c, &diags)
			d.Checks[k] = check
		}
	} else {
		d.Checks = nil
	}

	if c.Restart.Policy != "" {
		d.Restart = &TfRestart{
			Policy: types.String{Value: string(c.Restart.Policy)},
		}
		if d.Restart.Policy.Value == string(api.MachineRestartPolicyOnFailure) {
			d.Restart.MaxRetries = types.Int64{Value: int64(c.Restart.MaxRetries)}
		} else {
			d.Restart.MaxRetries = types.Int64{Null: true}
		}
	}
	return
}

func (d *flyMachineConfigData) toApiConfig(ctx context.Context, diags *diag.Diagnostics) *api.MachineConfig {
	conf := api.MachineConfig{
		Image:    d.Image.Value,
		Services: TfServicesToServices(d.Services),
		Init:     d.Init.toApi(),
	}

	if !d.Guest.IsNull() && !d.Guest.IsUnknown() {
		conf.Guest = &api.MachineGuest{
			CPUs:     int(d.Guest.Attrs["cpus"].(types.Int64).Value),
			CPUKind:  d.Guest.Attrs["cpu_kind"].(types.String).Value,
			MemoryMB: int(d.Guest.Attrs["memory_mb"].(types.Int64).Value),
		}
		diags.Append(d.Guest.Attrs["kernel_args"].(types.List).ElementsAs(ctx, &conf.Guest.KernelArgs, false)...)
		if diags.HasError() {
			return nil
		}
	}

	if !d.Env.Unknown {
		var env map[string]string
		diags.Append(d.Env.ElementsAs(context.Background(), &env, false)...)
		if diags.HasError() {
			return nil
		}
		conf.Env = env
	}

	if len(d.Mounts) > 0 {
		var mounts []api.MachineMount
		for _, m := range d.Mounts {
			mounts = append(mounts, api.MachineMount{
				//Encrypted: m.Encrypted.Value,
				Path: m.Path.Value,
				//SizeGb:    int(m.SizeGb.Value),
				Volume: m.Volume.Value,
			})
		}
		conf.Mounts = mounts
	}

	conf.Processes = make([]api.MachineProcess, len(d.Processes))
	for i, p := range d.Processes {
		var env map[string]string
		diags.Append(p.Env.ElementsAs(ctx, &env, false)...)
		diags.Append(d.Env.ElementsAs(context.Background(), &env, false)...)
		if diags.HasError() {
			return nil
		}
		conf.Processes[i] = api.MachineProcess{
			ExecOverride:       p.Exec,
			EntrypointOverride: p.Entrypoint,
			CmdOverride:        p.Cmd,
			UserOverride:       p.User.Value,
			ExtraEnv:           env,
		}
	}

	if d.Metrics != nil {
		conf.Metrics = &api.MachineMetrics{
			Port: int(d.Metrics.Port.Value),
			Path: d.Metrics.Path.Value,
		}
	}

	if d.Schedule != nil {
		conf.Schedule = *d.Schedule
	}

	conf.Checks = make(map[string]api.MachineCheck, len(d.Checks))
	for k, c := range d.Checks {
		ac := api.MachineCheck{
			Type: c.Type.Value,
			Port: uint16(c.Port.Value),
		}
		if c.Interval.IsUnknown() || c.Interval.IsNull() {
			ac.Interval = nil
		} else {
			ac.Interval = &api.Duration{Duration: *c.Interval.GetDuration()}
		}
		if c.Timeout.IsUnknown() || c.Timeout.IsNull() {
			ac.Timeout = nil
		} else {
			ac.Timeout = &api.Duration{Duration: *c.Timeout.GetDuration()}
		}
		if !c.HttpPath.IsNull() && !c.HttpPath.IsUnknown() {
			ac.HTTPPath = api.StringPointer(c.HttpPath.Value)
		}
		if !c.HttpMethod.IsNull() && !c.HttpMethod.IsUnknown() {
			ac.HTTPMethod = api.StringPointer(c.HttpMethod.Value)
		}
		conf.Checks[k] = ac
	}

	if d.Restart != nil {
		if !d.Restart.Policy.IsNull() && !d.Restart.Policy.IsUnknown() {
			conf.Restart.Policy = api.MachineRestartPolicy(d.Restart.Policy.Value)
		}
		if !d.Restart.MaxRetries.IsNull() && !d.Restart.MaxRetries.IsUnknown() {
			conf.Restart.MaxRetries = int(d.Restart.MaxRetries.Value)
		}
	}
	return &conf
}

// CreateMachine response is incomplete, so to avoid deleting data we only set selected fields here
func (d *flyMachineResourceData) updateFromApiCreateData(m api.Machine) (diags diag.Diagnostics) {
	d.Name = types.String{Value: m.Name}
	d.Id = types.String{Value: m.ID}
	d.PrivateIP = types.String{Value: m.PrivateIP}
	if m.Config == nil {
		diags.AddWarning("invalid API data received", "machine.config is nil")
		return
	}
	d.Config.updateFromApiCreateData(*m.Config)
	return
}

func (d *flyMachineResourceData) updateFromApiData(m api.Machine) (diags diag.Diagnostics) {
	diags.Append(d.updateFromApiCreateData(m)...)
	if diags.HasError() {
		return
	}
	d.Region = types.String{Value: m.Region}

	if m.Config == nil {
		diags.AddWarning("invalid API data received", "machine.config is nil")
		return
	}
	d.Config.updateFromApiData(*m.Config)
	return
}
