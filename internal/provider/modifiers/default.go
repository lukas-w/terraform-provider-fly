package modifiers

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// defaultModifier is a plan modifier that sets a default value for an attribute
// when it is not configured. The attribute must be marked as Optional and
// Computed. When setting the state during the resource Create, Read, or Update
// methods, this default value must also be included or the Terraform CLI will
// generate an error.
type defaultModifier struct {
	Default attr.Value
}

func (m defaultModifier) Description(context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to %s", m.Default)
}

func (m defaultModifier) MarkdownDescription(context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m.Default)
}

func (m defaultModifier) Modify(_ context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	if req.AttributeConfig.IsNull() && (req.AttributePlan.IsUnknown() || req.AttributePlan.IsNull()) {
		resp.AttributePlan = m.Default
	}
}

func Default(defaultValue attr.Value) tfsdk.AttributePlanModifier {
	return defaultModifier{
		Default: defaultValue,
	}
}
