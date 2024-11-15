package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/superfly/fly-go"
	"time"
)

func ToFlyDurationOrNil(d timetypes.GoDuration, diags *diag.Diagnostics) *fly.Duration {
	if d.IsNull() || d.IsUnknown() {
		return nil
	}

	duration, err := time.ParseDuration(d.ValueString())
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("Duration Parse Error", err.Error()))
		return nil
	}

	return &fly.Duration{Duration: duration}
}
