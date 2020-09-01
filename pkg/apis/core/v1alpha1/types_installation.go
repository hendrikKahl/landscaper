// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package v1alpha1

import (
	"encoding/json"

	cdv2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EncompassedByLabel is the label that contains the name of the parent installation
// that encompasses the current installation.
// todo: add conversion
const EncompassedByLabel = "landscaper.gardener.cloud/encompassed-by"

// todo: keep only subinstallations?
const KeepChildrenAnnotation = "landscaper.gardener.cloud/keep-children"

// EnsureSubInstallationsCondition is the Conditions type to indicate the sub installation status.
const EnsureSubInstallationsCondition ConditionType = "EnsureSubInstallations"

// CreateImportsCondition is the Conditions type to indicate status of the imported data and data objects.
const CreateImportsCondition ConditionType = "CreateImports"

// CreateExportsCondition is the Conditions type to indicate status of the exported data and data objects.
const CreateExportsCondition ConditionType = "CreateExports"

// EnsureExecutionsCondition is the Conditions type to indicate the executions status.
const EnsureExecutionsCondition ConditionType = "EnsureExecutions"

// ValidateExportCondition is the Conditions type to indicate validation status of teh exported data.
const ValidateExportCondition ConditionType = "ValidateExport"

type ComponentInstallationPhase string

const (
	ComponentPhaseInit        ComponentInstallationPhase = "Init"
	ComponentPhasePending     ComponentInstallationPhase = "PendingDependencies"
	ComponentPhaseProgressing ComponentInstallationPhase = "Progressing"
	ComponentPhaseDeleting    ComponentInstallationPhase = "Deleting"
	ComponentPhaseAborted     ComponentInstallationPhase = "Aborted"
	ComponentPhaseSucceeded   ComponentInstallationPhase = "Succeeded"
	ComponentPhaseFailed      ComponentInstallationPhase = "Failed"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InstallationList contains a list of Components
type InstallationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Installation `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Blueprint contains the configuration of a component
// +kubebuilder:resource:path="installations",scope="Namespaced",shortName="inst",singular="installation"
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Phase,type=string
// +kubebuilder:printcolumn:JSONPath=".status.configGeneration",name=ConfigGen,type=integer
// +kubebuilder:printcolumn:JSONPath=".status.executionRef.name",name=Execution,type=string
// +kubebuilder:printcolumn:JSONPath=".status.exportRef.name",name=ExportRef,type=string
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
// +kubebuilder:subresource:status
type Installation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InstallationSpec `json:"spec"`

	// +optional
	Status InstallationStatus `json:"status"`
}

// InstallationSpec defines a component installation.
type InstallationSpec struct {
	// BlueprintRef is the resolved reference to the definition.
	BlueprintRef RemoteBlueprintReference `json:"blueprintRef"`

	// RegistryPullSecrets defines a list of registry credentials that are used to
	// pull blueprints and component descriptors from the respective registry.
	// For more info see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
	// Note that the type information is used to determine the secret key and the type of the secret.
	// +optional
	RegistryPullSecrets []ObjectReference `json:"registryPullSecrets"`

	// Imports define the import mapping for the referenced definition.
	// These values are by default auto generated from the parent definition.
	// +optional
	Imports []DefinitionImportMapping `json:"imports,omitempty"`

	// Exports define the export mappings for the referenced definition.
	// These values are by default auto generated from the parent definition.
	// todo: add static data boolean
	// +optional
	Exports []DefinitionExportMapping `json:"exports,omitempty"`

	// StaticData contains a list of data sources that are used to satisfy imports
	// +optional
	StaticData []StaticDataSource `json:"staticData,omitempty"`
}

// InstallationStatus contains the current status of a Installation.
type InstallationStatus struct {
	// Phase is the current phase of the installation.
	Phase ComponentInstallationPhase `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed for this ControllerInstallations.
	// It corresponds to the ControllerInstallations generation, which is updated on mutation by the landscaper.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Conditions contains the actual condition of a installation
	Conditions []Condition `json:"conditions,omitempty"`

	// ConfigGeneration is the generation of the exported values.
	ConfigGeneration string `json:"configGeneration"`

	// ExportReference references the object that contains the exported values.
	ExportReference *ObjectReference `json:"exportRef,omitempty"`

	// ImportReference references the object that contains the temporary imported values.
	ImportReference *ObjectReference `json:"importRef,omitempty"`

	// Imports contain the state of the imported values.
	Imports []ImportState `json:"imports,omitempty"`

	// InstallationReferences contain all references to sub-components
	// that are created based on the component definition.
	InstallationReferences []NamedObjectReference `json:"installationRefs,omitempty"`

	// ExecutionReference is the reference to the execution that schedules the templated execution items.
	ExecutionReference *ObjectReference `json:"executionRef,omitempty"`
}

// RemoteBlueprintReference describes a reference to a blueprint defined by a component descriptor.
type RemoteBlueprintReference struct {
	VersionedResourceReference `json:",inline"`
	// RepositoryContext defines the context of the component repository to resolve blueprints.
	cdv2.RepositoryContext `json:",inline"`
}

// StaticDataSource defines a static data source
type StaticDataSource struct {
	// Value defined inline a raw data
	// +optional
	Value json.RawMessage `json:"value,omitempty"`

	// ValueFrom defines data from an external resource
	ValueFrom *StaticDataValueFrom `json:"valueFrom,omitempty"`
}

// StaticDataValueFrom defines a static data that is read from a external resource.
type StaticDataValueFrom struct {
	// Selects a key of a secret in the installations's namespace
	// +optional
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`

	// Selects a key from multiple secrets in the installations's namespace
	// that matches the given labels.
	// +optional
	SecretLabelSelector *SecretLabelSelectorRef `json:"secretLabelSelector,omitempty"`
}

// SecretLabelSelectorRef selects secrets with the given label and key.
type SecretLabelSelectorRef struct {
	// Selector is a map of labels to select specific secrets.
	Selector map[string]string `json:"selector"`

	// The key of the secret to select from.  Must be a valid secret key.
	Key string `json:"key"`
}

// ImportState hold the state of a import.
type ImportState struct {
	// From is the from key of the import
	// todo: maybe remove in favor of one key from the definition
	From string `json:"from"`

	// To is the to key of the import
	To string `json:"to"`

	// SourceRef is the reference to the installation where the value is imported
	SourceRef *ObjectReference `json:"sourceRef,omitempty"`

	// ConfigGeneration is the generation of the imported value.
	ConfigGeneration string `json:"configGeneration"`
}
