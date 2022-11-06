package provider

import (
	"context"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/fly-apps/terraform-provider-fly/internal/utils"
	"github.com/fly-apps/terraform-provider-fly/pkg/apiv1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	Port     types.Int64    `tfsdk:"port"`
	Handlers []types.String `tfsdk:"handlers"`
}

type TfService struct {
	Ports        []TfPort     `tfsdk:"ports"`
	Protocol     types.String `tfsdk:"protocol"`
	InternalPort types.Int64  `tfsdk:"internal_port"`
}

type flyMachineResourceData struct {
	Name       types.String `tfsdk:"name"`
	Region     types.String `tfsdk:"region"`
	Id         types.String `tfsdk:"id"`
	PrivateIP  types.String `tfsdk:"privateip"`
	App        types.String `tfsdk:"app"`
	Image      types.String `tfsdk:"image"`
	Cpus       types.Int64  `tfsdk:"cpus"`
	MemoryMb   types.Int64  `tfsdk:"memorymb"`
	CpuType    types.String `tfsdk:"cputype"`
	Env        types.Map    `tfsdk:"env"`
	Cmd        []string     `tfsdk:"cmd"`
	Entrypoint []string     `tfsdk:"entrypoint"`
	Exec       []string     `tfsdk:"exec"`

	Mounts   []TfMachineMount `tfsdk:"mounts"`
	Services []TfService      `tfsdk:"services"`
}

func (d *flyMachineResourceData) updateFromApiData(machine apiv1.MachineResponse) {
	d.Name = types.String{Value: machine.Name}
	d.Region = types.String{Value: machine.Region}
	d.Id = types.String{Value: machine.ID}
	d.PrivateIP = types.String{Value: machine.PrivateIP}
	d.Image = types.String{Value: machine.Config.Image}
	d.Cpus = types.Int64{Value: int64(machine.Config.Guest.Cpus)}
	d.MemoryMb = types.Int64{Value: int64(machine.Config.Guest.MemoryMb)}
	d.CpuType = types.String{Value: machine.Config.Guest.CPUKind}
	d.Cmd = machine.Config.Init.Cmd
	d.Entrypoint = machine.Config.Init.Entrypoint
	d.Exec = machine.Config.Init.Exec
	d.Env = utils.KVToTfMap(machine.Config.Env, types.StringType)

	tfServices := ServicesToTfServices(machine.Config.Services)
	if d.Services == nil && len(tfServices) == 0 {
		tfServices = nil
	}
	d.Services = tfServices

	if len(machine.Config.Mounts) > 0 {
		var mounts []TfMachineMount
		for _, m := range machine.Config.Mounts {
			mounts = append(mounts, TfMachineMount{
				Encrypted: types.Bool{Value: m.Encrypted},
				Path:      types.String{Value: m.Path},
				SizeGb:    types.Int64{Value: int64(m.SizeGb)},
				Volume:    types.String{Value: m.Volume},
			})
		}
		d.Mounts = mounts
	}
}

func (d *flyMachineResourceData) makeMachineRequest() apiv1.MachineCreateOrUpdateRequest {
	conf := apiv1.MachineConfig{
		Image:    d.Image.Value,
		Services: TfServicesToServices(d.Services),
		Init: apiv1.InitConfig{
			Cmd:        d.Cmd,
			Exec:       d.Exec,
			Entrypoint: d.Entrypoint,
		},
	}

	if !d.Cpus.Unknown {
		conf.Guest.Cpus = int(d.Cpus.Value)
	}
	if !d.CpuType.Unknown {
		conf.Guest.CpuType = apiv1.CpuType(d.CpuType.Value)
	}
	if !d.MemoryMb.Unknown {
		conf.Guest.MemoryMb = int(d.MemoryMb.Value)
	}

	if !d.Env.Unknown {
		var env map[string]string
		d.Env.ElementsAs(context.Background(), &env, false)
		conf.Env = env
	}

	if len(d.Mounts) > 0 {
		var mounts []apiv1.MachineMount
		for _, m := range d.Mounts {
			mounts = append(mounts, apiv1.MachineMount{
				Encrypted: m.Encrypted.Value,
				Path:      m.Path.Value,
				SizeGb:    int(m.SizeGb.Value),
				Volume:    m.Volume.Value,
			})
		}
		conf.Mounts = mounts
	}

	return apiv1.MachineCreateOrUpdateRequest{
		Name:   d.Name.Value,
		Region: d.Region.Value,
		Config: conf,
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
			"privateip": {
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
			"cputype": {
				MarkdownDescription: "cpu type",
				Computed:            true,
				Optional:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.String{Value: string(apiv1.CpuTypeShared)})},
			},
			"cpus": {
				MarkdownDescription: "cpu count",
				Computed:            true,
				Optional:            true,
				Type:                types.Int64Type,
				PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.Int64{Value: 1})},
				Validators:          []tfsdk.AttributeValidator{int64validator.AtLeast(1)},
			},
			"memorymb": {
				MarkdownDescription: "memory mb",
				Computed:            true,
				Optional:            true,
				Type:                types.Int64Type,
				PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.Int64{Value: 256})},
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
								Type:                types.ListType{ElemType: types.StringType},
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
				}),
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

func TfServicesToServices(input []TfService) []apiv1.Service {
	services := make([]apiv1.Service, 0)
	for _, s := range input {
		var ports []apiv1.Port
		for _, j := range s.Ports {
			var handlers []string
			for _, k := range j.Handlers {
				handlers = append(handlers, k.Value)
			}
			ports = append(ports, apiv1.Port{
				Port:     j.Port.Value,
				Handlers: handlers,
			})
		}
		services = append(services, apiv1.Service{
			Ports:        ports,
			Protocol:     s.Protocol.Value,
			InternalPort: s.InternalPort.Value,
		})
	}
	return services
}

func ServicesToTfServices(input []apiv1.Service) []TfService {
	tfservices := make([]TfService, 0)
	for _, s := range input {
		var tfports []TfPort
		for _, j := range s.Ports {
			var handlers []types.String
			for _, k := range j.Handlers {
				handlers = append(handlers, types.String{Value: k})
			}
			tfports = append(tfports, TfPort{
				Port:     types.Int64{Value: int64(j.Port)},
				Handlers: handlers,
			})
		}
		tfservices = append(tfservices, TfService{
			Ports:        tfports,
			Protocol:     types.String{Value: s.Protocol},
			InternalPort: types.Int64{Value: int64(s.InternalPort)},
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
	var machine apiv1.MachineResponse
	err := machineAPI.CreateMachine(data.makeMachineRequest(), data.App.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create machine", err.Error())
		return
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", machine))

	data.updateFromApiData(machine)

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

	var machine apiv1.MachineResponse
	_, err := machineAPI.ReadMachine(data.App.Value, data.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get machine", err.Error())
		return
	}

	data.updateFromApiData(machine)
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
	var machine apiv1.MachineResponse
	err := machineAPI.UpdateMachine(plan.makeMachineRequest(), state.App.Value, state.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update machine", err.Error())
		return
	}

	state.updateFromApiData(machine)

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
