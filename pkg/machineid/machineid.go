// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package machineid

import (
	"io/ioutil"
	"path/filepath"
	"strings"
)

// MachineID returns the uuid specified `/etc/machine-id`
// it first prepends the provided root to the filepath
// if there is an error reading the file it returns the empty string
func MachineID(root string) string {
	fullPath := filepath.Join(root, "/etc/machine-id")
	id, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(id))
}
