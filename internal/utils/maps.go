package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func KVToTfMap(kv map[string]string, elemType attr.Type, emptyIfNull bool) types.Map {
	elems := map[string]attr.Value{}
	if kv == nil {
		if !emptyIfNull {
			return types.MapNull(elemType)
		}
	} else {
		for key, value := range kv {
			elems[key] = types.StringValue(value)
		}
	}

	return types.MapValueMust(elemType, elems)
}

func StringSliceToTfList(slice []string) types.List {
	if slice == nil {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, len(slice))
	for i, v := range slice {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}
