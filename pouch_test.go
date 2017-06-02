/*
Copyright 2017 Tuenti Technologies S.L. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pouch

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
)

func TestPouchRun(t *testing.T) {
	vault := &DummyVault{
		t: t,

		ExpectedToken:    "token",
		ExpectedSecretID: "secret",

		RoleID:   "roleid",
		SecretID: "secret",

		Responses: map[string]*api.Secret{
			"GET/v1/foo": &api.Secret{
				Data: map[string]interface{}{"foo": "secretfoo", "bar": "secretbar"},
			},
		},
	}
	tmpdir, err := ioutil.TempDir("", "pouch-test")
	if err != nil {
		t.Fatalf("couldn't create temporal directory")
	}
	defer os.RemoveAll(tmpdir)
	secrets := []SecretConfig{
		{
			VaultURL:   "/v1/foo",
			HTTPMethod: "GET",
			Files: []FileConfig{
				{Path: path.Join(tmpdir, "foo"), Template: "{{ .foo }}"},
				{Path: path.Join(tmpdir, "bar"), Template: "{{ .bar }}"},
			},
		},
	}

	pouch := NewPouch(vault, secrets)

	err = pouch.Run()
	if err != nil {
		t.Fatal(err)
	}

	d, err := ioutil.ReadFile(path.Join(tmpdir, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretfoo", "File content should be the secret")

	d, err = ioutil.ReadFile(path.Join(tmpdir, "bar"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretbar", "File content should be the secret")
}

func TestPouchWatch(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "pouch-test")
	if err != nil {
		t.Fatalf("couldn't create temporal directory")
	}
	defer os.RemoveAll(tmpdir)

	secretWrapPath, err := ioutil.TempFile("", "pouch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(secretWrapPath.Name())

	vault := &DummyVault{
		t: t,

		ExpectedToken:    "token",
		ExpectedSecretID: "secret",
		WrappedSecretID:  "wrap",

		RoleID: "roleid",

		Responses: map[string]*api.Secret{
			"GET/v1/foo": &api.Secret{
				Data: map[string]interface{}{"foo": "secretfoo"},
			},
		},
	}
	secrets := []SecretConfig{
		{
			VaultURL:   "/v1/foo",
			HTTPMethod: "GET",
			Files: []FileConfig{
				{Path: path.Join(tmpdir, "foo"), Template: "{{ .foo }}"},
			},
		},
	}

	pouch := NewPouch(vault, secrets)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	err = w.Add(tmpdir)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		err := pouch.Watch(secretWrapPath.Name())
		if err != nil {
			t.Fatal(err)
		}
	}()

	secretWrapPath.Write([]byte("wrap"))
	secretWrapPath.Close()

	written := false
	for !written {
		select {
		case e := <-w.Events:
			written = (e.Op == fsnotify.Write) && (e.Name == path.Join(tmpdir, "foo"))
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	d, err := ioutil.ReadFile(path.Join(tmpdir, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, string(d), "secretfoo", "File content should be the secret")
}
