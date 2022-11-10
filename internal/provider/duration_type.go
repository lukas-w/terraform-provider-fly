package provider

import (
	"context"
	"errors"
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
		return DurationValue{Null: true}, nil
	}
	if !value.IsKnown() {
		return DurationValue{Unknown: true}, nil
	}
	var s string
	if err := value.As(&s); err != nil {
		return nil, err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}
	return DurationValue{Value: duration}, nil
}

func (d DurationType) ValueType(context.Context) attr.Value {
	return DurationValue{Null: true}
}

func (d DurationType) Equal(o attr.Type) bool {
	other, ok := o.(DurationType)
	if !ok {
		return false
	}
	return o == other
}

func (d DurationType) String() string {
	return "DurationType"
}

func (d DurationType) ApplyTerraform5AttributePathStep(step tftypes.AttributePathStep) (interface{}, error) {
	return nil, errors.New("can't apply attribute path step to DurationType")
}

func (d DurationType) Validate(_ context.Context, value tftypes.Value, path path.Path) (diags diag.Diagnostics) {
	var s string
	if err := value.As(&s); err != nil {
		diags.AddAttributeError(path, "invalid value", "can't use as string")
		return
	}
	_, err := time.ParseDuration(s)
	if err != nil {
		diags.AddAttributeError(path, "value is not a valid duration", err.Error())
	}
	return
}

type DurationValue struct {
	Value   time.Duration
	Null    bool
	Unknown bool
}

func (d DurationValue) Type(context.Context) attr.Type {
	return DurationType{}
}

func (d DurationValue) ToTerraformValue(context.Context) (tftypes.Value, error) {
	return tftypes.NewValue(tftypes.String, d.Value.String()), nil
}

func (d DurationValue) Equal(value attr.Value) bool {
	d2, err := time.ParseDuration(value.String())
	if err != nil {
		return false
	}
	return d.Value == d2
}

func (d DurationValue) IsNull() bool {
	return d.Null
}

func (d DurationValue) IsUnknown() bool {
	return d.Unknown
}

func (d DurationValue) String() string {
	return d.Value.String()
}
