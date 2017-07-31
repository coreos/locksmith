// Copyright 2017 CoreOS, Inc.
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

package filelock

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileLock(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "locksmith_filelock_test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)
	lockPath := filepath.Join(tmpDir, "lock")
	lockFile, err := os.Create(filepath.Join(tmpDir, "lock"))
	if err != nil {
		t.Fatalf("error creating test file:% v", err)
	}
	lockFile.Close()

	ulock, err := NewExclusiveLock(lockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ulock.Update(strings.NewReader("foo"))
	assertContents(t, lockPath, "foo")

	ulock.Update(strings.NewReader("bar"))
	assertContents(t, lockPath, "bar")

	_, err = NewExclusiveLock(lockPath)
	if err == nil {
		t.Fatalf("was able to make a new lock; expected lock to still be held")
	}

	if err := ulock.Unlock(); err != nil {
		t.Fatalf("could not unlock lock: %v", err)
	}

	_, err = NewExclusiveLock(lockPath)
	if err != nil {
		t.Fatalf("expected to be able to re-lock file after unlock: %v", err)
	}
}

func assertContents(t *testing.T, path string, contents string) {
	//t.Helper() once go 1.9 is the only supported version
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %v: %v", path, err)
	}
	if string(data) != contents {
		t.Errorf("expected %v to contain %v; was %v", path, contents, string(data))
	}
}
