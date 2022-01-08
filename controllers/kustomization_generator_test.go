/*
Copyright 2022 The Flux authors

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

package controllers

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fluxcd/kustomize-controller/api/v1beta2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const resourcePath = "./testdata/generator/resources/"

func BenchmarkKustomizationGenerator(b *testing.B) {
	b.StopTimer()
	g := NewWithT(b)

	f := func() {
		os.Remove("./testdata/generator/resources/kustomization.yaml")
	}

	b.Cleanup(f)

	// Create a kustomization file with varsub
	yamlKus, err := os.ReadFile("./testdata/generator/kustomization.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	var object v1beta2.Kustomization
	runtime.DefaultUnstructuredConverter.FromUnstructured(clientObjects[0].UnstructuredContent(), &object)
	g.Expect(err).NotTo(HaveOccurred())

	//Get a generator
	gen := NewGenerator(object)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err = gen.WriteFile(resourcePath)
		g.Expect(err).NotTo(HaveOccurred())
	}
}
func TestKustomizationGenerator(t *testing.T) {
	g := NewWithT(t)

	// Create a kustomization file with varsub
	yamlKus, err := os.ReadFile("./testdata/generator/kustomization.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	var object v1beta2.Kustomization
	runtime.DefaultUnstructuredConverter.FromUnstructured(clientObjects[0].UnstructuredContent(), &object)
	g.Expect(err).NotTo(HaveOccurred())

	//Get a generator
	gen := NewGenerator(object)
	err = gen.WriteFile(resourcePath)
	g.Expect(err).NotTo(HaveOccurred())

	// Get resource from directory
	fs := filesys.MakeFsOnDisk()
	resMap, err := buildKustomization(fs, resourcePath)
	g.Expect(err).NotTo(HaveOccurred())

	// Check that the resource has been substituted
	resources, err := resMap.AsYaml()
	g.Expect(err).NotTo(HaveOccurred())

	//load expected result
	expected, err := os.ReadFile("./testdata/generator/kustomization_expected.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(resources)).To(Equal(string(expected)))
}

func readYamlObjects(rdr io.Reader) ([]unstructured.Unstructured, error) {
	objects := []unstructured.Unstructured{}
	reader := k8syaml.NewYAMLReader(bufio.NewReader(rdr))
	for {
		doc, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		unstructuredObj := &unstructured.Unstructured{}
		decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(doc), len(doc))
		err = decoder.Decode(unstructuredObj)
		if err != nil {
			return nil, err
		}
		objects = append(objects, *unstructuredObj)
	}
	return objects, nil
}
