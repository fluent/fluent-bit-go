/* -*- Mode: C; tab-width: 4; indent-tabs-mode: nil; c-basic-offset: 4 -*- */

/*  Fluent Bit Go!
 *  ==============
 *  Copyright (C) 2015-2017 Treasure Data Inc.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

#ifndef FLBGO_OUTPUT_H
#define FLBGO_OUTPUT_H

#include <stdio.h>

struct flb_api {
    char *(*output_get_property) (char *, void *);
};

struct flbgo_output_plugin {
    char *name;
    struct flb_api *api;
    void *o_ins;
    int (*cb_init)();
    int (*cb_flush)(void *, size_t, char *);
    int (*cb_exit)(void *);
};

char *output_get_property(char *key, void *ctx)
{
    struct flbgo_output_plugin *plugin = ctx;
    return plugin->api->output_get_property(key, plugin->o_ins);
}

#endif
