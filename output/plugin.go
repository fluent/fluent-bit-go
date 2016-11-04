package flbout

/* #include "../c/flbgo_plugin.h" */
import "C"

import "unsafe"

/* Define constants matching Fluent Bit core */
const FLB_PROXY_OUTPUT_PLUGIN =  C.FLB_PROXY_OUTPUT_PLUGIN
const FLB_PROXY_GOLANG        =  C.FLB_PROXY_GOLANG

type FLBPlugin C.struct_flb_plugin_proxy

func CreatePlugin(name string, desc string) *FLBPlugin {
	p := (*FLBPlugin)(C.malloc(C.size_t(unsafe.Sizeof(FLBPlugin{}))))
	p.Type        = C.FLB_PROXY_OUTPUT_PLUGIN
	p.Flags       = 0
	p.Name        = C.CString(name)
	p.Description = C.CString(desc)

	return p;
}
