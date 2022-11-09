package provider

import (
	"context"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/fly-apps/terraform-provider-fly/pkg/apiv1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/flyctl/api"
	"time"
)

var _ resource.ResourceWithConfigure = &flyMachineResource{}
var _ resource.ResourceWithImportState = &flyMachineResource{}

type flyMachineResource struct {
	flyResource
}

func newFlyMachineResource() resource.Resource {
	return &flyMachineResource{}
}

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
	Type       types.String `tfsdk:"type"`
	Port       types.Int64  `tfsdk:"port"`
	Interval   types.String `tfsdk:"interval"`
	Timeout    types.String `tfsdk:"timeout"`
	HttpMethod types.String `tfsdk:"http_method"`
	HttpPath   types.String `tfsdk:"http_path"`
}

func checkFromApi(c api.MachineCheck) (r TfCheck) {
	r.Type = types.String{Value: c.Type}
	r.Port = types.Int64{Value: int64(c.Port)}
	r.Interval = types.String{Value: c.Interval.String()}
	r.Timeout = types.String{Value: c.Timeout.String()}
	r.HttpPath = types.String{Null: c.HTTPPath != nil}
	r.HttpMethod = types.String{Null: c.HTTPMethod != nil}
	if c.HTTPPath != nil {
		r.HttpPath.Value = *c.HTTPPath
	}
	if c.HTTPMethod != nil {
		r.HttpMethod.Value = *c.HTTPMethod
	}
	return
}

type flyMachineResourceData struct {
	Name       types.String `tfsdk:"name"`
	Region     types.String `tfsdk:"region"`
	Id         types.String `tfsdk:"id"`
	PrivateIP  types.String `tfsdk:"private_ip"`
	App        types.String `tfsdk:"app"`
	Image      types.String `tfsdk:"image"`
	Guest      types.Object `tfsdk:"guest"`
	Env        types.Map    `tfsdk:"env"`
	Cmd        []string     `tfsdk:"cmd"`
	Entrypoint []string     `tfsdk:"entrypoint"`
	Exec       []string     `tfsdk:"exec"`
	Metadata   types.Map    `tfsdk:"metadata"`

	Mounts    []TfMachineMount   `tfsdk:"mounts"`
	Services  []TfService        `tfsdk:"services"`
	Processes []TfProcess        `tfsdk:"processes"`
	Restart   *TfRestart         `tfsdk:"restart"`
	Metrics   *TfMetric          `tfsdk:"metrics"`
	Checks    map[string]TfCheck `tfsdk:"checks"`
}

func (d *flyMachineResourceData) updateFromApiData(m api.Machine) (diags diag.Diagnostics) {
	d.Name = types.String{Value: m.Name}
	d.Region = types.String{Value: m.Region}
	d.Id = types.String{Value: m.ID}
	d.PrivateIP = types.String{Value: m.PrivateIP}
	d.Image = types.String{Value: m.Config.Image}
	if m.Config.Guest != nil {
		d.Guest = types.Object{AttrTypes: TfGuestAttrTypes, Attrs: map[string]attr.Value{
			"cpus":        types.Int64{Value: int64(m.Config.Guest.CPUs)},
			"memory_mb":   types.Int64{Value: int64(m.Config.Guest.MemoryMB)},
			"cpu_kind":    types.String{Value: m.Config.Guest.CPUKind},
			"kernel_args": utils.StringSliceToTfList(m.Config.Guest.KernelArgs),
		}}
	}
	d.Cmd = m.Config.Init.Cmd
	d.Entrypoint = m.Config.Init.Entrypoint
	d.Exec = m.Config.Init.Exec
	d.Env = utils.KVToTfMap(m.Config.Env, types.StringType)
	d.Metadata = utils.KVToTfMap(m.Config.Metadata, types.StringType)

	tfServices := ServicesToTfServices(m.Config.Services)
	if d.Services == nil && len(tfServices) == 0 {
		tfServices = nil
	}
	d.Services = tfServices

	if len(m.Config.Mounts) > 0 {
		var mounts []TfMachineMount
		for _, m := range m.Config.Mounts {
			mounts = append(mounts, TfMachineMount{
				Encrypted: types.Bool{Value: m.Encrypted},
				Path:      types.String{Value: m.Path},
				SizeGb:    types.Int64{Value: int64(m.SizeGb)},
				Volume:    types.String{Value: m.Volume},
			})
		}
		d.Mounts = mounts
	}

	if len(m.Config.Processes) > 0 {
		d.Processes = make([]TfProcess, len(m.Config.Processes))
		for i, p := range m.Config.Processes {
			d.Processes[i] = TfProcess{
				Exec:       p.ExecOverride,
				Entrypoint: p.EntrypointOverride,
				Cmd:        p.CmdOverride,
				User:       types.String{Value: p.UserOverride},
				Env:        utils.KVToTfMap(p.ExtraEnv, types.StringType),
			}
		}
	} else {
		d.Processes = nil
	}

	if m.Config.Metrics != nil {
		d.Metrics = &TfMetric{
			Port: types.Int64{Value: int64(m.Config.Metrics.Port)},
			Path: types.String{Value: m.Config.Metrics.Path},
		}
	}

	if len(m.Config.Checks) > 0 {
		d.Checks = make(map[string]TfCheck)
		for i, c := range m.Config.Checks {
			d.Checks[i] = checkFromApi(c)
		}
	} else {
		d.Checks = nil
	}

	if m.Config.Restart.Policy != "" {
		d.Restart = &TfRestart{
			Policy:     types.String{Value: string(m.Config.Restart.Policy)},
			MaxRetries: types.Int64{Value: int64(m.Config.Restart.MaxRetries)},
		}
	}
	return
}

func (d *flyMachineResourceData) makeMachineRequest(ctx context.Context, diags diag.Diagnostics) *api.LaunchMachineInput {
	conf := api.MachineConfig{
		Image:    d.Image.Value,
		Services: TfServicesToServices(d.Services),
		Init: api.MachineInit{
			Cmd:        d.Cmd,
			Exec:       d.Exec,
			Entrypoint: d.Entrypoint,
		},
	}

	if !d.Guest.IsNull() && !d.Guest.IsUnknown() {
		conf.Guest = &api.MachineGuest{
			CPUs:     int(d.Guest.Attrs["cpus"].(types.Int64).Value),
			CPUKind:  d.Guest.Attrs["cpu_king"].(types.String).Value,
			MemoryMB: int(d.Guest.Attrs["cpus"].(types.Int64).Value),
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
				Encrypted: m.Encrypted.Value,
				Path:      m.Path.Value,
				SizeGb:    int(m.SizeGb.Value),
				Volume:    m.Volume.Value,
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

	conf.Checks = make(map[string]api.MachineCheck, len(d.Checks))
	for k, c := range d.Checks {
		interval, err := time.ParseDuration(c.Interval.Value)
		if err != nil {
			diags.AddError("Can't parse interval", err.Error())
		}
		timeout, err := time.ParseDuration(c.Timeout.Value)
		if err != nil {
			diags.AddError("Can't parse timeout", err.Error())
		}
		ac := api.MachineCheck{
			Type:     c.Type.Value,
			Port:     uint16(c.Port.Value),
			Interval: &api.Duration{Duration: interval},
			Timeout:  &api.Duration{Duration: timeout},
		}
		if !c.HttpPath.IsNull() && !c.HttpPath.IsUnknown() {
			ac.HTTPPath = &c.HttpPath.Value
		}
		if !c.HttpMethod.IsNull() && !c.HttpMethod.IsUnknown() {
			ac.HTTPMethod = &c.HttpMethod.Value
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

	return &api.LaunchMachineInput{
		Name:   d.Name.Value,
		Region: d.Region.Value,
		Config: &conf,
	}
}

type TfMachineMount struct {
	Encrypted types.Bool   `tfsdk:"encrypted"`
	Path      types.String `tfsdk:"path"`
	SizeGb    types.Int64  `tfsdk:"size_gb"`
	Volume    types.String `tfsdk:"volume"`
}

func (mr flyMachineResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_machine"
}

func (flyMachineResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Fly machine resource",
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				MarkdownDescription: "machine name",
				Optional:            true,
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
				Type: types.StringType,
			},
			"region": {
				MarkdownDescription: "machine region",
				Required:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
				},
				Type: types.StringType,
			},
			"id": {
				MarkdownDescription: "machine id",
				Computed:            true,
				Type:                types.StringType,
			},
			"app": {
				MarkdownDescription: "fly app",
				Optional:            true,
				Type:                types.StringType,
			},
			"private_ip": {
				MarkdownDescription: "Private IP",
				Computed:            true,
				Type:                types.StringType,
			},
			"cmd": {
				MarkdownDescription: "cmd",
				Optional:            true,
				//Computed:            true,
				Type: types.ListType{ElemType: types.StringType},
			},
			"entrypoint": {
				MarkdownDescription: "image entrypoint",
				Optional:            true,
				//Computed:            true,
				Type: types.ListType{ElemType: types.StringType},
			},
			"exec": {
				MarkdownDescription: "exec command",
				Optional:            true,
				//Computed:            true,
				Type: types.ListType{ElemType: types.StringType},
			},
			"image": {
				MarkdownDescription: "docker image",
				Required:            true,
				Type:                types.StringType,
			},
			"guest": {
				Optional: true,
				Computed: true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"cpu_kind": {
						MarkdownDescription: "cpu type",
						Required:            true,
						Type:                types.StringType,
						PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.String{Value: string(apiv1.CpuTypeShared)})},
					},
					"cpus": {
						MarkdownDescription: "cpu count",
						Required:            true,
						Type:                types.Int64Type,
						PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.Int64{Value: 1})},
						Validators:          []tfsdk.AttributeValidator{int64validator.AtLeast(1)},
					},
					"memory_mb": {
						MarkdownDescription: "memory mb",
						Required:            true,
						Type:                types.Int64Type,
						PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.Int64{Value: 256})},
					},
					"kernel_args": {
						Optional: true,
						Computed: true,
						Type:     types.ListType{ElemType: types.StringType},
					},
				}),
			},
			"env": {
				MarkdownDescription: "Optional environment variables, keys and values must be strings",
				Optional:            true,
				Computed:            true,
				Type:                types.MapType{ElemType: types.StringType},
			},
			"mounts": {
				MarkdownDescription: "Volume mounts",
				Optional:            true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"encrypted": {
						Optional: true,
						Computed: true,
						Type:     types.BoolType,
					},
					"path": {
						Required:            true,
						MarkdownDescription: "Path for volume to be mounted on vm",
						Type:                types.StringType,
					},
					"size_gb": {
						Optional: true,
						Computed: true,
						Type:     types.Int64Type,
					},
					"volume": {
						Required:            true,
						MarkdownDescription: "Name or ID of volume",
						Type:                types.StringType,
					},
				}),
			},
			"services": {
				MarkdownDescription: "services",
				Optional:            true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"ports": {
						MarkdownDescription: "External ports and handlers",
						Required:            true,
						Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
							"port": {
								MarkdownDescription: "External port",
								Required:            true,
								Type:                types.Int64Type,
							},
							"handlers": {
								MarkdownDescription: "How the edge should process requests",
								Optional:            true,
								Computed:            true,
								Type:                types.ListType{ElemType: types.StringType},
							},
							"force_https": {
								Type:          types.BoolType,
								Optional:      true,
								Computed:      true,
								PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Bool{Value: false})},
							},
						}),
					},
					"protocol": {
						MarkdownDescription: "network protocol",
						Required:            true,
						Type:                types.StringType,
					},
					"internal_port": {
						MarkdownDescription: "Port application listens on internally",
						Required:            true,
						Type:                types.Int64Type,
					},
					"concurrency": {
						Optional: true,
						Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
							"type": {
								Type:        types.StringType,
								Description: "Specifies what metric to use to measure load for the application. Either \"connections\" or \"requests\"",
								Required:    true,
							},
							"hard_limit": {
								Type:     types.Int64Type,
								Required: true,
							},
							"soft_limit": {
								Type:     types.Int64Type,
								Required: true,
							},
						}),
					},
				}),
			},
			"processes": {
				Optional: true,
				Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
					"exec": {
						Optional: true,
						Type:     types.ListType{ElemType: types.StringType},
					},
					"entrypoint": {
						Optional: true,
						Type:     types.ListType{ElemType: types.StringType},
					},
					"cmd": {
						Optional: true,
						Type:     types.ListType{ElemType: types.StringType},
					},
					"user": {
						Optional: true,
						Type:     types.StringType,
					},
					"env": {
						Optional: true,
						Type:     types.MapType{ElemType: types.StringType},
					},
				}),
			},
			"restart": {
				Optional: true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"policy": {
						Required: true,

						Type:                types.StringType,
						Description:         "Restart policy, one of \"no\", \"on-failure\" and \"always\"",
						MarkdownDescription: "Restart policy, one of `\"no\"`, `\"on-failure\"` and `\"always\"`",
					},
					"max_retries": {
						Optional:            true,
						Type:                types.Int64Type,
						Description:         "Only relevant with the \"on-failure\" policy",
						MarkdownDescription: "Only relevant with the `\"on-failure\"` policy",
					},
				}),
			},
			"metrics": {
				Optional: true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"port": {
						Type:     types.Int64Type,
						Required: true,
					},
					"path": {
						Type:     types.StringType,
						Required: true,
					},
				}),
			},
			"checks": {
				Optional: true,
				Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
					"type": {
						Type:     types.StringType,
						Required: true,
					},
					"port": {
						Type:     types.Int64Type,
						Optional: true,
					},
					"interval": {
						Type:     types.StringType,
						Optional: true,
					},
					"timeout": {
						Type:     types.StringType,
						Optional: true,
					},
					"http_method": {
						Type:     types.StringType,
						Optional: true,
					},
					"http_path": {
						Type:     types.StringType,
						Optional: true,
					},
				}),
			},
			"metadata": {
				Optional: true,
				Computed: true,
				Type:     types.MapType{ElemType: types.StringType},
			},
		},
	}, nil
}

func (mr flyMachineResource) ValidateOpenTunnel(diags diag.Diagnostics) bool {
	_, err := mr.httpClient.R().Get(fmt.Sprintf("http://%s", mr.httpEndpoint))
	if err != nil {
		diags.AddError("fly wireguard tunnel must be open", "can't connect to the api, is the tunnel open? :)")
		return false
	}
	return true
}

func TfServicesToServices(input []TfService) []api.MachineService {
	services := make([]api.MachineService, 0)
	for _, s := range input {
		var ports []api.MachinePort
		for _, j := range s.Ports {
			var handlers []string
			for _, k := range j.Handlers {
				handlers = append(handlers, k.Value)
			}
			ports = append(ports, api.MachinePort{
				Port:       int(j.Port.Value),
				Handlers:   handlers,
				ForceHttps: j.ForceHttps.Value,
			})
		}
		var concurrency *api.MachineServiceConcurrency
		if s.Concurrency != nil {
			concurrency = &api.MachineServiceConcurrency{
				Type:      s.Concurrency.Type.Value,
				HardLimit: int(s.Concurrency.HardLimit.Value),
				SoftLimit: int(s.Concurrency.SoftLimit.Value),
			}
		}
		services = append(services, api.MachineService{
			Ports:        ports,
			Protocol:     s.Protocol.Value,
			InternalPort: int(s.InternalPort.Value),
			Concurrency:  concurrency,
		})
	}
	return services
}

func ServicesToTfServices(input []api.MachineService) []TfService {
	tfservices := make([]TfService, 0)
	for _, s := range input {
		var tfports []TfPort
		for _, j := range s.Ports {
			var handlers []types.String
			for _, k := range j.Handlers {
				handlers = append(handlers, types.String{Value: k})
			}
			tfports = append(tfports, TfPort{
				Port:       types.Int64{Value: int64(j.Port)},
				Handlers:   handlers,
				ForceHttps: types.Bool{Value: j.ForceHttps},
			})
		}
		var concurrency *TfServiceConcurrency
		if s.Concurrency != nil {
			concurrency = &TfServiceConcurrency{
				Type:      types.String{Value: s.Concurrency.Type},
				HardLimit: types.Int64{Value: int64(s.Concurrency.HardLimit)},
				SoftLimit: types.Int64{Value: int64(s.Concurrency.SoftLimit)},
			}
		}
		tfservices = append(tfservices, TfService{
			Ports:        tfports,
			Protocol:     types.String{Value: s.Protocol},
			InternalPort: types.Int64{Value: int64(s.InternalPort)},
			Concurrency:  concurrency,
		})
	}
	return tfservices
}

func (mr flyMachineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !mr.ValidateOpenTunnel(resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machineAPI := apiv1.NewMachineAPI(&mr.httpClient, mr.httpEndpoint)
	mReq := data.makeMachineRequest(ctx, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var machine api.Machine
	err := machineAPI.CreateMachine(*mReq, data.App.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create machine", err.Error())
		return
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", machine))

	resp.Diagnostics.Append(data.updateFromApiData(machine)...)

	err = machineAPI.WaitForMachine(data.App.Value, machine.ID, machine.InstanceID)
	if err != nil {
		//FIXME(?): For now we just assume that the orchestrator is in fact going to faithfully execute our request
		tflog.Info(ctx, "Waiting errored")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !mr.ValidateOpenTunnel(resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	machineAPI := apiv1.NewMachineAPI(&mr.httpClient, mr.httpEndpoint)

	var machine api.Machine
	_, err := machineAPI.ReadMachine(data.App.Value, data.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get machine", err.Error())
		return
	}

	resp.Diagnostics.Append(data.updateFromApiData(machine)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !mr.ValidateOpenTunnel(resp.Diagnostics) {
		return
	}

	var plan, state flyMachineResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machineAPI := apiv1.NewMachineAPI(&mr.httpClient, mr.httpEndpoint)
	mReq := plan.makeMachineRequest(ctx, resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	var machine api.Machine
	err := machineAPI.UpdateMachine(*mReq, state.App.Value, state.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update machine", err.Error())
		return
	}

	resp.Diagnostics.Append(state.updateFromApiData(machine)...)

	err = machineAPI.WaitForMachine(state.App.Value, machine.ID, machine.InstanceID)
	if err != nil {
		tflog.Info(ctx, "Waiting errored")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (mr flyMachineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !mr.ValidateOpenTunnel(resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	machineAPI := apiv1.NewMachineAPI(&mr.httpClient, mr.httpEndpoint)

	err := machineAPI.DeleteMachine(data.App.Value, data.Id.Value, 50)
	if err != nil {
		resp.Diagnostics.AddError("Machine delete failed", err.Error())
	}
}

func (mr flyMachineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
