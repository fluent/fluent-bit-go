//  Fluent Bit Go!
//  ==============
//  Copyright (C) 2015-2017 Treasure Data Inc.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package output

import (
	"testing"
	"unsafe"
)

func TestSetConfigMapDefValue(t *testing.T) {
	tests := []struct {
		name     string
		cmap     ConfigMap
		wantNull bool
	}{
		{
			name:     "bool without default",
			cmap:     ConfigMap{Type: FLB_CONFIG_MAP_BOOL, Name: "verbose", Desc: "Enable verbose mode"},
			wantNull: true,
		},
		{
			name:     "prefixed string without default",
			cmap:     ConfigMap{Type: FLB_CONFIG_MAP_STR_PREFIX, Name: "header_", Desc: "Extra headers"},
			wantNull: true,
		},
		{
			name:     "int with default",
			cmap:     ConfigMap{Type: FLB_CONFIG_MAP_INT, Name: "retries", DefValue: "3", Desc: "Retry count"},
			wantNull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var def FLBPluginProxyDef
			FLBPluginRegisterWithOptions(unsafe.Pointer(&def),
				WithName("test"),
				WithDescription("test plugin"),
				WithConfigMap([]ConfigMap{tt.cmap}),
			)
			defer FLBPluginUnregister(unsafe.Pointer(&def))

			if def.config_map == nil {
				t.Fatal("config_map is NULL, want an entry")
			}
			if gotNull := def.config_map.def_value == nil; gotNull != tt.wantNull {
				t.Errorf("def_value is NULL = %v, want %v", gotNull, tt.wantNull)
			}
		})
	}
}
