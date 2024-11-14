package provider

import (
	"context"
	"fmt"
	"github.com/fly-apps/terraform-provider-fly/internal/provider/modifiers"
	"github.com/fly-apps/terraform-provider-fly/pkg/apiv1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/fly-go"
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

func (d *flyMachineResourceData) makeMachineRequest(ctx context.Context, diags *diag.Diagnostics) *fly.LaunchMachineInput {
	conf := d.Config.toApiConfig(ctx, diags)
	if diags.HasError() {
		return nil
	}
	if conf == nil {
		diags.AddError("error creating machine request", "Config is nil. This is a bug.")
		return nil
	}
	return &fly.LaunchMachineInput{
		Name:   d.Name.Value,
		Region: d.Region.Value,
		Config: conf,
	}
}

func (mr flyMachineResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_machine"
}

func (flyMachineResource) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		MarkdownDescription: "Fly machine resource",
		Version:             1,
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				MarkdownDescription: "machine name",
				Optional:            true,
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.RequiresReplace(),
					resource.UseStateForUnknown(),
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
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"app": {
				MarkdownDescription: "fly app",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers:       tfsdk.AttributePlanModifiers{resource.RequiresReplace()},
			},
			"private_ip": {
				MarkdownDescription: "Private IP",
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers:       []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
			},
			"config": {
				Required: true,
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"init": {
						Optional: true,
						Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
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
						}),
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
								PlanModifiers: []tfsdk.AttributePlanModifier{
									modifiers.Default(types.Int64{Value: 1}),
								},
								Validators: []tfsdk.AttributeValidator{int64validator.AtLeast(1)},
							},
							"memory_mb": {
								MarkdownDescription: "memory mb",
								Required:            true,
								Type:                types.Int64Type,
								PlanModifiers:       []tfsdk.AttributePlanModifier{modifiers.Default(types.Int64{Value: 256})},
							},
							"kernel_args": {
								Optional: true,
								Type:     types.ListType{ElemType: types.StringType},
								PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.List{
									Null:     true,
									ElemType: types.StringType,
								})},
							},
						}),
						PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
					},
					"env": {
						MarkdownDescription: "Optional environment variables, keys and values must be strings",
						Optional:            true,
						Computed:            true,
						Type:                types.MapType{ElemType: types.StringType},
						PlanModifiers: []tfsdk.AttributePlanModifier{
							modifiers.Default(types.Map{ElemType: types.StringType, Elems: map[string]attr.Value{}}),
							resource.UseStateForUnknown(),
						},
					},
					"mounts": {
						MarkdownDescription: "Volume mounts",
						Optional:            true,
						Attributes: tfsdk.ListNestedAttributes(map[string]tfsdk.Attribute{
							//"encrypted": {
							//	Optional:      true,
							//	Computed:      true,
							//	Type:          types.BoolType,
							//	PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
							//},
							"path": {
								Required:            true,
								MarkdownDescription: "Path for volume to be mounted on vm",
								Type:                types.StringType,
							},
							//"size_gb": {
							//	Optional:      true,
							//	Computed:      true,
							//	Type:          types.Int64Type,
							//	PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
							//},
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
						Computed: true,
						PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Object{
							AttrTypes: map[string]attr.Type{
								"policy":      types.StringType,
								"max_retries": types.Int64Type,
							},
							Null: true,
						})},
						Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
							"policy": {
								Required:            true,
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
					"schedule": {
						Optional:            true,
						Type:                types.StringType,
						Description:         "Schedule this machine. Options are \"hourly\", \"daily\", \"weekly\" and \"monthly\".",
						MarkdownDescription: "Schedule this machine. Options are `\"hourly\"`, `\"daily\"`, `\"weekly\"` and `\"monthly\"`.",
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
								Type:     DurationType{},
								Optional: true,
							},
							"timeout": {
								Type:     DurationType{},
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
						Optional:      true,
						Computed:      true,
						Type:          types.MapType{ElemType: types.StringType},
						PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Map{ElemType: types.StringType, Elems: map[string]attr.Value{}})},
					},
				}),
			},
			"wait": {
				Optional:      true,
				Computed:      true,
				Type:          types.BoolType,
				Description:   "Wait for machine to start up",
				PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Bool{Value: true})},
			},
			"wait_for_checks": {
				Optional:      true,
				Computed:      true,
				Type:          types.BoolType,
				Description:   "Wait for machine's health checks to pass",
				PlanModifiers: []tfsdk.AttributePlanModifier{modifiers.Default(types.Bool{Value: false})},
			},
		},
	}, nil
}

func (mr flyMachineResource) ValidateOpenTunnel(diags *diag.Diagnostics) bool {
	_, err := mr.httpClient.R().Get(fmt.Sprintf("http://%s", mr.httpEndpoint))
	if err != nil {
		diags.AddError("fly wireguard tunnel must be open", "can't connect to the api, is the tunnel open? :)")
		return false
	}
	return true
}

func TfServicesToServices(input []TfService) []fly.MachineService {
	services := make([]fly.MachineService, 0)
	for _, s := range input {
		var ports []fly.MachinePort
		for _, j := range s.Ports {
			var handlers []string
			for _, k := range j.Handlers {
				handlers = append(handlers, k.Value)
			}
			port := int(j.Port.Value)
			ports = append(ports, fly.MachinePort{
				Port:       &port,
				Handlers:   handlers,
				ForceHTTPS: j.ForceHttps.Value,
			})
		}
		var concurrency *fly.MachineServiceConcurrency
		if s.Concurrency != nil {
			concurrency = &fly.MachineServiceConcurrency{
				Type:      s.Concurrency.Type.Value,
				HardLimit: int(s.Concurrency.HardLimit.Value),
				SoftLimit: int(s.Concurrency.SoftLimit.Value),
			}
		}
		services = append(services, fly.MachineService{
			Ports:        ports,
			Protocol:     s.Protocol.Value,
			InternalPort: int(s.InternalPort.Value),
			Concurrency:  concurrency,
		})
	}
	return services
}

func ServicesToTfServices(input []fly.MachineService) []TfService {
	tfservices := make([]TfService, 0)
	for _, s := range input {
		var tfports []TfPort
		for _, j := range s.Ports {
			var handlers []types.String
			for _, k := range j.Handlers {
				handlers = append(handlers, types.String{Value: k})
			}
			tfports = append(tfports, TfPort{
				Port:       types.Int64{Value: int64(*j.Port)},
				Handlers:   handlers,
				ForceHttps: types.Bool{Value: j.ForceHTTPS},
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
	if !mr.ValidateOpenTunnel(&resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machineAPI := mr.machineApi(data.App.Value)
	mReq := data.makeMachineRequest(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if mReq == nil {
		resp.Diagnostics.AddError("error creating machine request", "Request is nil. This is a bug.")
		return
	}
	var machine fly.Machine
	err := machineAPI.CreateMachine(*mReq, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create machine", err.Error())
		return
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", machine))

	resp.Diagnostics.Append(data.updateFromApiCreateData(machine)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Wait.Equal(types.Bool{Value: true}) {
		err = machineAPI.WaitForMachine(machine.ID, machine.InstanceID)
		if err != nil {
			//FIXME(?): For now we just assume that the orchestrator is in fact going to faithfully execute our request
			tflog.Info(ctx, "Waiting errored")
		}
		if data.WaitForChecks.Equal(types.Bool{Value: true}) {
			resp.Diagnostics.Append(mr.WaitForChecks(ctx, data, &resp.State)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !mr.ValidateOpenTunnel(&resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	machineAPI := mr.machineApi(data.App.Value)

	var machine fly.Machine
	_, err := machineAPI.ReadMachine(data.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get machine", err.Error())
		return
	}

	resp.Diagnostics.Append(data.updateFromApiData(machine)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !mr.ValidateOpenTunnel(&resp.Diagnostics) {
		return
	}

	var plan, state flyMachineResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	machineAPI := mr.machineApi(state.App.Value)
	mReq := plan.makeMachineRequest(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if mReq == nil {
		resp.Diagnostics.AddError("error creating machine request", "Request is nil. This is a bug.")
		return
	}
	var machine fly.Machine
	err := machineAPI.UpdateMachine(*mReq, state.Id.Value, &machine)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update machine", err.Error())
		return
	}

	resp.Diagnostics.Append(state.updateFromApiData(machine)...)
	if plan.Wait.Equal(types.Bool{Value: true}) {
		err = machineAPI.WaitForMachine(machine.ID, machine.InstanceID)
		if err != nil {
			tflog.Info(ctx, "Waiting errored")
		}
		if plan.WaitForChecks.Equal(types.Bool{Value: true}) {
			resp.Diagnostics.Append(mr.WaitForChecks(ctx, state, &resp.State)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}
	state.WaitForChecks = plan.WaitForChecks

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (mr flyMachineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !mr.ValidateOpenTunnel(&resp.Diagnostics) {
		return
	}

	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	machineAPI := mr.machineApi(data.App.Value)

	err := machineAPI.DeleteMachine(data.Id.Value, 50)
	if err != nil {
		resp.Diagnostics.AddError("Machine delete failed", err.Error())
	}
}

func (mr flyMachineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (mr flyMachineResource) WaitForChecks(ctx context.Context, data flyMachineResourceData, state *tfsdk.State) (diags diag.Diagnostics) {
	interval := 5 * time.Second
	timeout := 5 * time.Minute
	maxErrors := 3

	machineApi := mr.machineApi(data.App.Value)

	timeoutCh := time.After(timeout)

	var machine fly.Machine
	var err error
	var failingChecks []*fly.MachineCheckStatus

	numErrors := 0
	for numErrors < maxErrors {
		select {
		case <-timeoutCh:
			diags.AddError("timed out while waiting for checks to pass", fmt.Sprintf("failing checks: %v", failingChecks))
			return
		case <-ctx.Done():
			diags.AddError("context cancelled", "")
			return
		case <-time.After(interval):
		}

		_, err = machineApi.ReadMachine(data.Id.Value, &machine)
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("ReadMachine failed while waiting for checks: %v", err.Error()))
			numErrors++
		} else {
			data.updateFromApiData(machine)
			diags.Append(state.Set(ctx, data)...)
			if diags.HasError() {
				return
			}

			failingChecks = make([]*fly.MachineCheckStatus, 0)
			for _, c := range machine.Checks {
				if c.Status != "passing" {
					failingChecks = append(failingChecks, c)
					tflog.Trace(ctx, "waiting for health check to pass", map[string]interface{}{
						"name":       c.Name,
						"status":     c.Status,
						"output":     c.Output,
						"updated_at": c.UpdatedAt,
					})
				}
			}
			if len(failingChecks) == 0 {
				return
			}
		}
	}
	diags.AddError("too many API errors while waiting for health checks", fmt.Sprintf("last error: %s", err.Error()))
	return
}

func allChecksPassing(checks []*fly.MachineCheckStatus) bool {
	for _, c := range checks {
		if c.Status != "passing" {
			return false
		}
	}
	return true
}
