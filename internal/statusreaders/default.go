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

package statusreaders

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	kstatusreaders "github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/kustomize-controller/internal/cel"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type customGenericStatusReader struct {
	genericStatusReader engine.StatusReader
	gvk                 schema.GroupVersionKind
}

func NewCustomGenericStatusReader(mapper meta.RESTMapper, gvk schema.GroupVersionKind, exprs map[string]string) engine.StatusReader {
	genericStatusReader := kstatusreaders.NewGenericStatusReader(mapper, genericConditions(gvk.Kind, exprs))
	return &customGenericStatusReader{
		genericStatusReader: genericStatusReader,
		gvk:                 gvk,
	}
}

func (g *customGenericStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == g.gvk.GroupKind()
}

func (g *customGenericStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	return g.genericStatusReader.ReadStatus(ctx, reader, resource)
}

func (g *customGenericStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return g.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
}

// We are expecting the following definition in the CR:
// customHealthChecks:
//   - apiVersion: "cert-manager.io/v1"
//     kind: "Certificate"
//     inProgress: "self.status.conditions.filter(e, e.type == 'Issuing').all(e, e.observedGeneration == self.metadata.generation && e.status == 'True')" # Match issuing condition.
//     failed: "self.status.conditions.filter(e, e.type == 'Ready').all(e, e.observedGeneration == self.metadata.generation && e.status == 'False')" # Match ready condition.
//     current: "self.status.conditions.filter(e, e.type == 'Ready').all(e, e.observedGeneration == self.metadata.generation && e.status == 'True')" # Match ready condition.
func genericConditions(kind string, exprs map[string]string) func(u *unstructured.Unstructured) (*status.Result, error) {
	return func(u *unstructured.Unstructured) (*status.Result, error) {
		obj := u.UnstructuredContent()

		// exprs are evaluated in order, so we can return the first match.
		for statusKey, expr := range exprs {
			eval, err := cel.Eval(expr, obj)
			if err != nil {
				return nil, err
			}
			switch statusKey {
			case status.CurrentStatus.String():
				if eval.Result {
					message := fmt.Sprintf("%s Succeeded", kind)
					return &status.Result{
						Status:     status.CurrentStatus,
						Message:    message,
						Conditions: []status.Condition{},
					}, nil
				}
			case status.FailedStatus.String():
				if eval.Result {
					message := fmt.Sprintf("%s Failed", kind)
					return &status.Result{
						Status:  status.FailedStatus,
						Message: message,
						Conditions: []status.Condition{
							{
								Type:    status.ConditionStalled,
								Status:  corev1.ConditionTrue,
								Reason:  fmt.Sprintf("% sFailed", kind),
								Message: message,
							},
						},
					}, nil
				}
			case status.InProgressStatus.String():
				if eval.Result {
					message := fmt.Sprintf("%s in progress", kind)
					return &status.Result{
						Status:  status.InProgressStatus,
						Message: message,
						Conditions: []status.Condition{
							{
								Type:    status.ConditionReconciling,
								Status:  corev1.ConditionTrue,
								Reason:  fmt.Sprintf("%s InProgress", kind),
								Message: message,
							},
						},
					}, nil
				}
			}
		}

		message := fmt.Sprintf("%s in progress", kind)
		return &status.Result{
			Status:  status.InProgressStatus,
			Message: message,
			Conditions: []status.Condition{
				{
					Type:    status.ConditionReconciling,
					Status:  corev1.ConditionTrue,
					Reason:  fmt.Sprintf("%s InProgress", kind),
					Message: message,
				},
			},
		}, nil
	}
}
