# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

CMDS=zfsplugin
all: build

include release-tools/build.make

.PHONY: code-generator
code-generator:
	podman run -it -v ${PWD}:/go/src/github.com/maxaudron/zfs-csi-driver cocainefarm/zfs-csi-driver:generator /bin/bash -c "cp -r /go/src/github.com/maxaudron/zfs-csi-driver/vendor/* /go/src/; ./generate-groups.sh all github.com/maxaudron/zfs-csi-driver/pkg/client github.com/maxaudron/zfs-csi-driver/pkg/apis zfsvolume:v1"
