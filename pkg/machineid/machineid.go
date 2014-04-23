package machineid

import (
	"io/ioutil"
	"path/filepath"
	"strings"
)

func MachineID(root string) string {
	fullPath := filepath.Join(root, "/etc/machine-id")
	id, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(id))
}
