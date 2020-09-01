// Copyright 2020 Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package installations

import (
	"context"
	"fmt"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	lsv1alpha1 "github.com/gardener/landscaper/pkg/apis/core/v1alpha1"
	lsv1alpha1helper "github.com/gardener/landscaper/pkg/apis/core/v1alpha1/helper"
	lscheme "github.com/gardener/landscaper/pkg/kubernetes"
	"github.com/gardener/landscaper/pkg/landscaper/blueprints"
	lsoperation "github.com/gardener/landscaper/pkg/landscaper/operation"
	"github.com/gardener/landscaper/pkg/landscaper/registry/components"
	"github.com/gardener/landscaper/pkg/utils/kubernetes"
)

var componentInstallationGVK schema.GroupVersionKind

func init() {
	var err error
	componentInstallationGVK, err = apiutil.GVKForObject(&lsv1alpha1.Installation{}, lscheme.LandscaperScheme)
	runtime.Must(err)
}

// IsRootInstallation returns if the installation is a root element.
func IsRootInstallation(inst *lsv1alpha1.Installation) bool {
	_, isOwned := kubernetes.OwnerOfGVK(inst.OwnerReferences, componentInstallationGVK)
	return !isOwned
}

// GetParentInstallationName returns the name of parent installation that encompasses the given installation.
func GetParentInstallationName(inst *lsv1alpha1.Installation) string {
	name, _ := kubernetes.OwnerOfGVK(inst.OwnerReferences, componentInstallationGVK)
	return name
}

// CreateInternalInstallations creates internal installations for a list of ComponentInstallations
func CreateInternalInstallations(ctx context.Context, op lsoperation.Interface, installations ...*lsv1alpha1.Installation) ([]*Installation, error) {
	internalInstallations := make([]*Installation, len(installations))
	for i, inst := range installations {
		inInst, err := CreateInternalInstallation(ctx, op, inst)
		if err != nil {
			return nil, err
		}
		internalInstallations[i] = inInst
	}
	return internalInstallations, nil
}

// ResolveComponentDescriptor resolves the component descriptor of a installation.
func ResolveComponentDescriptor(ctx context.Context, compRepo componentsregistry.Registry, inst *lsv1alpha1.Installation) (*cdv2.ComponentDescriptor, error) {
	return compRepo.Resolve(ctx, inst.Spec.BlueprintRef.RepositoryContext, inst.Spec.BlueprintRef.ObjectMeta())
}

// CreateInternalInstallation creates an internal installation for a Installation
func CreateInternalInstallation(ctx context.Context, op lsoperation.Interface, inst *lsv1alpha1.Installation) (*Installation, error) {
	blue, err := blueprints.Resolve(ctx, op, inst.Spec.BlueprintRef)
	if err != nil {
		return nil, fmt.Errorf("unable to resolve blueprint for %s/%s: %w", inst.Namespace, inst.Name, err)
	}
	return New(inst, blue)
}

// CheckCompletedSiblingDependentsOfParent checks if siblings and siblings of the parent's parents that the parent depends on (imports data) are completed.
func CheckCompletedSiblingDependentsOfParent(ctx context.Context, op lsoperation.Interface, parent *Installation) (bool, error) {
	if parent == nil {
		return true, nil
	}
	siblingsCompleted, err := CheckCompletedSiblingDependents(ctx, op, parent)
	if err != nil {
		return false, err
	}
	if !siblingsCompleted {
		return siblingsCompleted, nil
	}

	// check its own parent
	parentsParent, err := GetParent(ctx, op, parent)
	if err != nil {
		return false, errors.Wrap(err, "unable to get parent of parent")
	}

	if parentsParent == nil {
		return true, nil
	}
	return CheckCompletedSiblingDependentsOfParent(ctx, op, parentsParent)
}

// CheckCompletedSiblingDependents checks if siblings that the installation depends on (imports data) are completed
func CheckCompletedSiblingDependents(ctx context.Context, op lsoperation.Interface, inst *Installation) (bool, error) {
	if inst == nil {
		return true, nil
	}
	// todo: may not depend on status of object
	for _, impState := range inst.ImportStatus().GetStates() {
		if impState.SourceRef != nil {

			// check if the import is imported from myself or the parent
			// and continue if so as we have a different check for the parent
			if lsv1alpha1helper.ReferenceIsObject(*impState.SourceRef, inst.Info) {
				continue
			}

			parent, err := GetParent(ctx, op, inst)
			if err != nil {
				return false, err
			}
			if parent != nil && lsv1alpha1helper.ReferenceIsObject(*impState.SourceRef, parent.Info) {
				continue
			}

			// we expect that the source ref is always a installation
			inst := &lsv1alpha1.Installation{}
			if err := op.Client().Get(ctx, impState.SourceRef.NamespacedName(), inst); err != nil {
				return false, err
			}

			if !lsv1alpha1helper.IsCompletedInstallationPhase(inst.Status.Phase) {
				op.Log().V(3).Info("dependent installation not completed", "inst", impState.SourceRef.NamespacedName().String())
				return false, nil
			}

			intInst, err := CreateInternalInstallation(ctx, op, inst)
			if err != nil {
				return false, err
			}

			isCompleted, err := CheckCompletedSiblingDependents(ctx, op, intInst)
			if err != nil {
				return false, err
			}
			if !isCompleted {
				return false, nil
			}
		}
	}

	return true, nil
}
