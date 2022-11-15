package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"time"
)

var _ attr.Type = DurationType{}

type DurationType struct {
}

func (d DurationType) TerraformType(ctx context.Context) tftypes.Type {
	return tftypes.String
}

func (d DurationType) ValueFromTerraform(ctx context.Context, value tftypes.Value) (attr.Value, error) {
	if value.IsNull() {
		return DurationNull(), nil
	}
	if !value.IsKnown() {
		return DurationAttrValue{unknown: true}, nil
	}
	var s string
	if err := value.As(&s); err != nil {
		return nil, err
	}
	// XXX: Is this right?
	if s == "" {
		return DurationNull(), nil
	}
	_, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}
	return DurationAttrValue{value: s}, nil
}

func (d DurationType) ValueType(context.Context) attr.Value {
	return DurationNull()
}

func (d DurationType) Equal(o attr.Type) bool {
	_, ok := o.(DurationType)
	return ok
}

func (d DurationType) String() string {
	return "DurationType"
}

func (d DurationType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return nil, errors.New("can't apply attribute path step to DurationType")
}

func (d DurationType) Validate(_ context.Context, value tftypes.Value, path path.Path) (diags diag.Diagnostics) {
	if value.IsNull() || !value.IsKnown() {
		return
	}
	var s string
	if err := value.As(&s); err != nil {
		diags.AddAttributeError(path, "invalid value", "can't use as string")
		return
	}
	if s == "" {
		return
	}
	_, err := time.ParseDuration(s)
	if err != nil {
		diags.AddAttributeError(path, fmt.Sprintf("value at %s is not a valid duration", path.String()), err.Error())
	}
	return
}

type DurationAttrValue struct {
	value   string
	unknown bool
}

func (d DurationAttrValue) Type(context.Context) attr.Type {
	return DurationType{}
}

func (d DurationAttrValue) ToTerraformValue(context.Context) (tftypes.Value, error) {
	return tftypes.NewValue(tftypes.String, d.value), nil
}

func (d DurationAttrValue) EqualDuration(o time.Duration) bool {
	if d.unknown || d.IsNull() {
		return false
	}
	return *d.GetDuration() == o
}

func (d DurationAttrValue) GetDuration() *time.Duration {
	if d.unknown || d.IsNull() {
		return nil
	}
	dur, err := time.ParseDuration(d.value)
	if err != nil {
		panic("DurationAttrValue holds invalid duration string")
	}
	return &dur
}

func (d DurationAttrValue) Equal(value attr.Value) bool {
	if d.unknown != value.IsUnknown() {
		return false
	}
	if d.IsNull() != value.IsNull() {
		return false
	}
	if d.unknown || d.IsNull() {
		return true
	}
	d2, err := time.ParseDuration(value.String())
	if err != nil {
		return false
	}
	d1, err := time.ParseDuration(d.value)
	if err != nil {
		return false
	}
	return d1 == d2
}

func (d DurationAttrValue) IsNull() bool {
	return d.value == "" && !d.unknown
}

func (d DurationAttrValue) IsUnknown() bool {
	return d.unknown
}

func (d DurationAttrValue) String() string {
	return d.value
}

func DurationNull() DurationAttrValue {
	return DurationAttrValue{}
}

func DurationValueFromString(duration string, diags *diag.Diagnostics) DurationAttrValue {
	_, err := time.ParseDuration(duration)
	if err != nil {
		diags.AddError("invalid duration value", err.Error())
		return DurationNull()
	}
	return DurationAttrValue{value: duration}
}
func DurationValueFromDuration(duration time.Duration) DurationAttrValue {
	return DurationAttrValue{value: duration.String()}
}
