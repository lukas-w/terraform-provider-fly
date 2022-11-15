package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func KVToTfMap(kv map[string]string, elemType attr.Type, emptyIfNull bool) (tfMap types.Map) {
	tfMap.ElemType = elemType
	tfMap.Elems = map[string]attr.Value{}
	if kv == nil {
		if !emptyIfNull {
			tfMap.Null = true
		}
		return
	}
	for key, value := range kv {
		tfMap.Elems[key] = types.String{Value: value}
	}
	return
}

func StringSliceToTfList(slice []string) (list types.List) {
	list.ElemType = types.StringType
	if slice == nil {
		list.Null = true
		return
	}
	list.Elems = make([]attr.Value, len(slice))
	for i, v := range slice {
		list.Elems[i] = types.String{Value: v}
	}
	return
}
