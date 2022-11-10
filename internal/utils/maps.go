package utils

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func KVToTfMap(kv map[string]string, elemType attr.Type) types.Map {
	var TFMap types.Map
	TFMap.ElemType = elemType
	for key, value := range kv {
		if TFMap.Elems == nil {
			TFMap.Elems = map[string]attr.Value{}
		}
		TFMap.Elems[key] = types.String{Value: value}
	}
	return TFMap
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
