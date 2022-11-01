package validators

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type jsonValidator struct{}

func (j jsonValidator) Description(context.Context) string {
	return "Must be valid JSON"
}

func (j jsonValidator) MarkdownDescription(ctx context.Context) string {
	return j.Description(ctx)
}

func (j jsonValidator) Validate(ctx context.Context, request tfsdk.ValidateAttributeRequest, response *tfsdk.ValidateAttributeResponse) {
	if !json.Valid([]byte(request.AttributeConfig.String())) {
		response.Diagnostics.AddAttributeError(
			request.AttributePath,
			"Invalid JSON",
			"",
		)
	}
}

func JSON() tfsdk.AttributeValidator {
	return jsonValidator{}
}
