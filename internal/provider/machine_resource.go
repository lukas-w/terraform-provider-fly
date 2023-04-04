package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/superfly/fly-go"
	"github.com/superfly/fly-go/flaps"
	"strings"
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
		Name:   d.Name.ValueString(),
		Region: d.Region.ValueString(),
		Config: conf,
	}
}

func (mr flyMachineResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "fly_machine"
}

func (flyMachineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fly machine resource",
		Version:             1,
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "machine name",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "machine region",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "machine id",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"app": schema.StringAttribute{
				MarkdownDescription: "fly app",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"private_ip": schema.StringAttribute{
				MarkdownDescription: "Private IP",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"config": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"init": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"cmd": schema.ListAttribute{
								MarkdownDescription: "cmd",
								Optional:            true,
								//Computed:            true,
								ElementType: types.StringType,
							},
							"entrypoint": schema.ListAttribute{
								MarkdownDescription: "image entrypoint",
								Optional:            true,
								//Computed:            true,
								ElementType: types.StringType,
							},
							"exec": schema.ListAttribute{
								MarkdownDescription: "exec command",
								Optional:            true,
								//Computed:            true,
								ElementType: types.StringType,
							},
						},
					},
					"image": schema.StringAttribute{
						MarkdownDescription: "docker image",
						Required:            true,
					},
					"guest": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"cpu_kind": schema.StringAttribute{
								MarkdownDescription: "cpu type",
								Required:            true,
							},
							"cpus": schema.Int64Attribute{
								MarkdownDescription: "cpu count",
								Required:            true,
								Validators:          []validator.Int64{int64validator.AtLeast(1)},
							},
							"memory_mb": schema.Int64Attribute{
								MarkdownDescription: "memory mb",
								Required:            true,
							},
							"kernel_args": schema.ListAttribute{
								ElementType: types.StringType,
								Optional:    true,
								Computed:    true,
								Default:     listdefault.StaticValue(types.ListNull(types.StringType)),
							},
						},
						PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
					},
					"env": schema.MapAttribute{
						MarkdownDescription: "Optional environment variables, keys and values must be strings",
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
						Default: mapdefault.StaticValue(
							types.MapValueMust(types.StringType, map[string]attr.Value{}),
						),
						PlanModifiers: []planmodifier.Map{
							mapplanmodifier.UseStateForUnknown(),
						},
					},
					"mounts": schema.ListNestedAttribute{
						MarkdownDescription: "Volume mounts",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								//"encrypted": {
								//	Optional:      true,
								//	Computed:      true,
								//	Type:          types.BoolType,
								//	PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
								//},
								"path": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Path for volume to be mounted on vm",
								},
								//"size_gb": {
								//	Optional:      true,
								//	Computed:      true,
								//	Type:          types.Int64Type,
								//	PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
								//},
								"volume": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "Name or ID of volume",
								},
							},
						},
					},
					"services": schema.ListNestedAttribute{
						MarkdownDescription: "services",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"ports": schema.ListNestedAttribute{
									MarkdownDescription: "External ports and handlers",
									Required:            true,
									NestedObject: schema.NestedAttributeObject{
										Attributes: map[string]schema.Attribute{
											"port": schema.Int64Attribute{
												MarkdownDescription: "External port",
												Required:            true,
											},
											"handlers": schema.ListAttribute{
												MarkdownDescription: "How the edge should process requests",
												Optional:            true,
												Computed:            true,
												ElementType:         types.StringType,
											},
											"force_https": schema.BoolAttribute{
												Optional: true,
												Computed: true,
												Default:  booldefault.StaticBool(false),
											},
										},
									},
								},
								"protocol": schema.StringAttribute{
									MarkdownDescription: "network protocol",
									Required:            true,
								},
								"internal_port": schema.Int64Attribute{
									MarkdownDescription: "Port application listens on internally",
									Required:            true,
								},
								"concurrency": schema.SingleNestedAttribute{
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"type": schema.StringAttribute{
											Description: "Specifies what metric to use to measure load for the application. Either \"connections\" or \"requests\"",
											Required:    true,
										},
										"hard_limit": schema.Int64Attribute{
											Required: true,
										},
										"soft_limit": schema.Int64Attribute{
											Required: true,
										},
									},
								},
							},
						},
					},
					"processes": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"exec": schema.ListAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
								"entrypoint": schema.ListAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
								"cmd": schema.ListAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
								"user": schema.StringAttribute{
									Optional: true,
								},
								"env": schema.MapAttribute{
									Optional:    true,
									ElementType: types.StringType,
								},
							},
						},
					},
					"restart": schema.SingleNestedAttribute{
						Optional: true,
						Computed: true,
						Default: objectdefault.StaticValue(types.ObjectNull(
							map[string]attr.Type{
								"policy":      types.StringType,
								"max_retries": types.Int64Type,
							},
						)),
						Attributes: map[string]schema.Attribute{
							"policy": schema.StringAttribute{
								Required:            true,
								Description:         "Restart policy, one of \"no\", \"on-failure\" and \"always\"",
								MarkdownDescription: "Restart policy, one of `\"no\"`, `\"on-failure\"` and `\"always\"`",
							},
							"max_retries": schema.Int64Attribute{
								Optional:            true,
								Description:         "Only relevant with the \"on-failure\" policy",
								MarkdownDescription: "Only relevant with the `\"on-failure\"` policy",
							},
						},
					},
					"metrics": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"port": schema.Int64Attribute{
								Required: true,
							},
							"path": schema.StringAttribute{
								Required: true,
							},
						},
					},
					"schedule": schema.StringAttribute{
						Optional:            true,
						Description:         "Schedule this machine. Options are \"hourly\", \"daily\", \"weekly\" and \"monthly\".",
						MarkdownDescription: "Schedule this machine. Options are `\"hourly\"`, `\"daily\"`, `\"weekly\"` and `\"monthly\"`.",
					},
					"checks": schema.MapNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Required: true,
								},
								"port": schema.Int64Attribute{
									Optional: true,
								},
								"interval": schema.StringAttribute{
									CustomType: timetypes.GoDurationType{},
									Optional:   true,
								},
								"timeout": schema.StringAttribute{
									CustomType: timetypes.GoDurationType{},
									Optional:   true,
								},
								"http_method": schema.StringAttribute{
									Optional: true,
								},
								"http_path": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
					"metadata": schema.MapAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Default:     mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
					},
				},
			},
			"wait": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Wait for machine to start up",
				Default:     booldefault.StaticBool(true),
			},
			"wait_for_checks": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Wait for machine's health checks to pass",
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

func TfServicesToServices(input []TfService) []fly.MachineService {
	services := make([]fly.MachineService, 0)
	for _, s := range input {
		var ports []fly.MachinePort
		for _, j := range s.Ports {
			var handlers []string
			for _, k := range j.Handlers {
				handlers = append(handlers, k.ValueString())
			}
			port := int(j.Port.ValueInt64())
			ports = append(ports, fly.MachinePort{
				Port:       &port,
				Handlers:   handlers,
				ForceHTTPS: j.ForceHttps.ValueBool(),
			})
		}
		var concurrency *fly.MachineServiceConcurrency
		if s.Concurrency != nil {
			concurrency = &fly.MachineServiceConcurrency{
				Type:      s.Concurrency.Type.ValueString(),
				HardLimit: int(s.Concurrency.HardLimit.ValueInt64()),
				SoftLimit: int(s.Concurrency.SoftLimit.ValueInt64()),
			}
		}
		services = append(services, fly.MachineService{
			Ports:        ports,
			Protocol:     s.Protocol.ValueString(),
			InternalPort: int(s.InternalPort.ValueInt64()),
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
				handlers = append(handlers, types.StringValue(k))
			}
			tfports = append(tfports, TfPort{
				Port:       types.Int64Value(int64(*j.Port)),
				Handlers:   handlers,
				ForceHttps: types.BoolValue(j.ForceHTTPS),
			})
		}
		var concurrency *TfServiceConcurrency
		if s.Concurrency != nil {
			concurrency = &TfServiceConcurrency{
				Type:      types.StringValue(s.Concurrency.Type),
				HardLimit: types.Int64Value(int64(s.Concurrency.HardLimit)),
				SoftLimit: types.Int64Value(int64(s.Concurrency.SoftLimit)),
			}
		}
		tfservices = append(tfservices, TfService{
			Ports:        tfports,
			Protocol:     types.StringValue(s.Protocol),
			InternalPort: types.Int64Value(int64(s.InternalPort)),
			Concurrency:  concurrency,
		})
	}
	return tfservices
}

func (mr flyMachineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data flyMachineResourceData

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := mr.getFlapsClient(ctx, data.App.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	mReq := data.makeMachineRequest(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if mReq == nil {
		resp.Diagnostics.AddError("error creating machine request", "Request is nil. This is a bug.")
		return
	}
	machine, err := client.Launch(ctx, *mReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create machine", err.Error())
		return
	}

	tflog.Info(ctx, fmt.Sprintf("%+v", machine))

	resp.Diagnostics.Append(data.updateFromApiCreateData(*machine)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Wait.Equal(types.BoolValue(true)) {
		err = client.Wait(ctx, machine, fly.MachineStateStarted, 5*time.Minute)
		if err != nil {
			//FIXME(?): For now we just assume that the orchestrator is in fact going to faithfully execute our request
			tflog.Info(ctx, "Waiting errored")
		}
		if data.WaitForChecks.Equal(types.BoolValue(true)) {
			resp.Diagnostics.Append(mr.WaitForChecks(ctx, data, &resp.State)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	client := mr.getFlapsClient(ctx, data.App.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	machine, err := client.Get(ctx, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get machine", err.Error())
		return
	}

	resp.Diagnostics.Append(data.updateFromApiData(*machine)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (mr flyMachineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state flyMachineResourceData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := mr.getFlapsClient(ctx, state.App.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	mReq := plan.makeMachineRequest(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if mReq == nil {
		resp.Diagnostics.AddError("error creating machine request", "Request is nil. This is a bug.")
		return
	}
	mReq.ID = state.Id.ValueString()

	ttl := 30
	lease, err := client.AcquireLease(ctx, state.Id.ValueString(), &ttl)
	if err != nil {
		resp.Diagnostics.AddError("Failed to acquire lease", err.Error())
		return
	}

	machine, err := client.Update(ctx, *mReq, lease.Data.Nonce)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update machine", err.Error())
		return
	}

	resp.Diagnostics.Append(state.updateFromApiData(*machine)...)
	if plan.Wait.Equal(types.BoolValue(true)) {
		err = client.Wait(ctx, machine, fly.MachineStateStarted, 5*time.Minute)
		if err != nil {
			tflog.Info(ctx, "Waiting errored")
		}
		if plan.WaitForChecks.Equal(types.BoolValue(true)) {
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
	var data flyMachineResourceData
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	client := mr.getFlapsClient(ctx, data.App.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(stopAndDestroyMachine(ctx, *client, data.Id.ValueString())...)
}

func stopAndDestroyMachine(ctx context.Context, client flaps.Client, id string) (diag diag.Diagnostics) {
	ttl := 60
	lease, err := client.AcquireLease(ctx, id, &ttl)
	if err != nil {
		diag.AddError("Failed to acquire lease", err.Error())
		return
	}
	nonce := lease.Data.Nonce

	machine := stopMachine(ctx, client, id, nonce, 5*time.Minute, diag)
	if diag.HasError() {
		return
	}

	err = client.Destroy(ctx, fly.RemoveMachineInput{
		ID: id,
	}, nonce)
	if err != nil {
		diag.AddError("Machine delete failed", err.Error())
		return
	}

	// Wait for destroyed
	err = client.Wait(ctx, machine, fly.MachineStateDestroyed, 1*time.Minute)
	if err != nil {
		diag.AddError("Failed to wait for machine to be destroyed", err.Error())
		return
	}

	machine, err = client.Get(ctx, id)
	if err != nil {
		diag.AddError("Failed to get machine", err.Error())
		return
	}
	if machine.State != fly.MachineStateDestroyed {
		diag.AddError("Failed to destroy machine", "Waited for machine to be destroyed but it's still there")
		return
	}

	return
}

func stopMachine(ctx context.Context, client flaps.Client, id string, nonce string, timeout time.Duration, diag diag.Diagnostics) (machine *fly.Machine) {
	machine, err := client.Get(ctx, id)
	if err != nil {
		diag.AddError("Failed to get machine", err.Error())
		return
	}

	if machine.State == fly.MachineStateStarted {
		err = client.Stop(ctx, fly.StopMachineInput{
			ID: id,
		}, nonce)
		if err != nil {
			diag.AddError("Failed to stop machine", err.Error())
			return
		}

		// Wait for machine to be stopped
		err = client.Wait(ctx, machine, fly.MachineStateStopped, timeout)
		if err != nil {
			diag.AddError("Failed to wait for machine to stop", err.Error())
			return
		}

		machine, err = client.Get(ctx, id)
		if err != nil {
			diag.AddError("Failed to get machine", err.Error())
			return
		}

		if machine.State == fly.MachineStateStarted {
			diag.AddError("Failed to stop machine", "Waited for machine to stop but it's still running")
			return
		}
	} else {
		tflog.Info(ctx, "Machine is already stopped")
	}
	return
}

func (mr flyMachineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: app_id,machine_id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("app"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), idParts[1])...)
}

func (mr flyMachineResource) WaitForChecks(ctx context.Context, data flyMachineResourceData, state *tfsdk.State) (diags diag.Diagnostics) {
	interval := 5 * time.Second
	timeout := 5 * time.Minute
	maxErrors := 3

	client := mr.getFlapsClient(ctx, data.App.ValueString(), &diags)
	if diags.HasError() {
		return
	}

	timeoutCh := time.After(timeout)

	var machine *fly.Machine
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

		machine, err = client.Get(ctx, data.Id.ValueString())
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("ReadMachine failed while waiting for checks: %v", err.Error()))
			numErrors++
		} else {
			data.updateFromApiData(*machine)
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
