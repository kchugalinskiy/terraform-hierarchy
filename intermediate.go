package main

import (
	log "github.com/Sirupsen/logrus"
)

// arguments/inputs
type ResourceArgumentUsage struct {
	Arg       *ResourceArgument `form:"Arg" json:"Arg" xml:"Arg"`
	UsagePath [][]string        `form:"UsagePath" json:"UsagePath" xml:"UsagePath"`
}

type ModuleInputUsage struct {
	Input     *ModuleInstance
	UsagePath [][]string `form:"UsagePath" json:"UsagePath" xml:"UsagePath"`
}

type ModuleInput struct {
	Name          string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded      bool
	AsArgument    []ResourceArgumentUsage `form:"AsArgument" json:"AsArgument" xml:"AsArgument"`
	AsModuleInput []ModuleInputUsage      `form:"AsModuleInput" json:"AsModuleInput" xml:"AsModuleInput"`
}

// attributes/outputs
type ResourceAttributeUsage struct {
	Attr *ResourceAttribute `form:"Arg" json:"Arg" xml:"Arg"`
}

type ModuleOutputUsage struct {
	Input *ModuleInstance `form:"Input" json:"Input" xml:"Input"`
}

type ModuleOutput struct {
	Name             string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded         bool
	FromAttribute    []ResourceAttributeUsage `form:"FromAttribute" json:"FromAttribute" xml:"FromAttribute"`
	FromModuleOutput []ModuleOutputUsage      `form:"FromModuleOutput" json:"FromModuleOutput" xml:"FromModuleOutput"`
}

// modules
type ModuleInstance struct {
	InstanceName string  `form:"InstanceName" json:"InstanceName" xml:"InstanceName"`
	Instance     *Module `form:"-" json:"-" xml:"-"`
}

type Module struct {
	Name            string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded        bool
	ModuleInstances []ModuleInstance `form:"ModuleInstances" json:"ModuleInstances" xml:"ModuleInstances"`
	Inputs          []*ModuleInput   `form:"Inputs" json:"Inputs" xml:"Inputs"`
	Outputs         []*ModuleOutput  `form:"Outputs" json:"Outputs" xml:"Outputs"`
}

// The state
type HierarchyState struct {
	AllModules []Module `form:"AllModules" json:"AllModules" xml:"AllModules"`
	allInputs  []ModuleInput
	allOutputs []ModuleOutput

	allModulesMap map[string]*Module
	allInputsMap  map[string]*ModuleInput
	allOutputsMap map[string]*ModuleOutput
}

func NewHierarchyState() *HierarchyState {
	return &HierarchyState{
		allModulesMap: make(map[string]*Module),
		allInputsMap:  make(map[string]*ModuleInput),
		allOutputsMap: make(map[string]*ModuleOutput),
	}
}

type VariableID string

type ResourceFieldID struct {
	Name         string
	InstanceName string
	FieldName    string
}

type ModuleFieldID struct {
	InstanceName string
	FieldName    string
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Module methods
func (h *HierarchyState) NewModule(name string) *Module {
	m, found := h.allModulesMap[name]
	if !found {
		h.AllModules = append(h.AllModules, Module{Name: name, IsLoaded: false})
		m = &h.AllModules[len(h.AllModules)-1]
		h.allModulesMap[name] = m
	}
	return m
}

func (m *Module) FindModuleInstance(instanceName string) *ModuleInstance {
	for _, instance := range m.ModuleInstances {
		if instance.InstanceName == instanceName {
			return &instance
		}
	}
	return nil
}

func (m *Module) NewInstance(instanceName string, instance *Module) {
	if nil == m.FindModuleInstance(instanceName) {
		m.ModuleInstances = append(m.ModuleInstances, ModuleInstance{Instance: instance, InstanceName: instanceName})
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Module inputs
func (h *HierarchyState) NewInput(module *Module, id VariableID) *ModuleInput {
	name := string(id)
	inputKey := module.Name + "." + name
	input, found := h.allInputsMap[inputKey]
	if !found {
		h.allInputs = append(h.allInputs, ModuleInput{Name: name, IsLoaded: false})
		input = &h.allInputs[len(h.allInputs)-1]
		h.allInputsMap[inputKey] = input
		module.Inputs = append(module.Inputs, input)
	}
	return input
}

func (m *ModuleInput) AttachArgument(usagePath []string, argument *ResourceArgument) {
	for _, elem := range m.AsArgument {
		if elem.Arg == argument {
			return
		}
	}

	m.AsArgument = append(m.AsArgument, ResourceArgumentUsage{Arg: argument, UsagePath: [][]string{usagePath}})
}

func (m *ModuleInput) AttachModuleInput(usagePath []string, instance *ModuleInstance) {
	for _, elem := range m.AsModuleInput {
		if elem.Input == instance {
			return
		}
	}

	m.AsModuleInput = append(m.AsModuleInput, ModuleInputUsage{Input: instance, UsagePath: [][]string{usagePath}})
}

func (h *HierarchyState) ConnectInputToArgument(module *Module, id VariableID, usagePath []string, argument *ResourceArgument) {
	log.Debugf("module %v name %v attach argument %v", module.Name, id, argument)
	value := h.NewInput(module, id)
	value.AttachArgument(usagePath, argument)
}

func (h *HierarchyState) ConnectInputToModuleInput(module *Module, id VariableID, usagePath []string, instance *ModuleInstance) {
	log.Debugf("module %v name %v attach input %v", module.Name, id, instance)
	value := h.NewInput(module, id)
	value.AttachModuleInput(usagePath, instance)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Module outputs

func (h *HierarchyState) NewOutput(module *Module, id VariableID) *ModuleOutput {
	name := string(id)
	outputKey := module.Name + "." + name
	output, found := h.allOutputsMap[outputKey]
	if !found {
		h.allOutputs = append(h.allOutputs, ModuleOutput{Name: name, IsLoaded: false})
		output = &h.allOutputs[len(h.allOutputs)-1]
		h.allOutputsMap[outputKey] = output
		module.Outputs = append(module.Outputs, output)
	}
	return output
}

func (m *ModuleOutput) AttachAttribute(attribute *ResourceAttribute) {
	for _, elem := range m.FromAttribute {
		if elem.Attr == attribute {
			return
		}
	}

	m.FromAttribute = append(m.FromAttribute, ResourceAttributeUsage{Attr: attribute})
}

func (m *ModuleOutput) AttachModuleOutput(instance *ModuleInstance) {
	for _, elem := range m.FromModuleOutput {
		if elem.Input == instance {
			return
		}
	}

	m.FromModuleOutput = append(m.FromModuleOutput, ModuleOutputUsage{Input: instance})
}

func (h *HierarchyState) ConnectOutputToAttribute(module *Module, id VariableID, attribute *ResourceAttribute) {
	log.Debugf("module %v name %v attach attribute %v", module.Name, id, attribute)
	value := h.NewOutput(module, id)
	value.AttachAttribute(attribute)
}

func (h *HierarchyState) ConnectOutputToModuleOutput(instance *ModuleInstance, id VariableID, moduleFieldUsage ModuleFieldID) {
	log.Debugf("instance %v name %v attach module output %v", instance, moduleFieldUsage)
	value := h.NewOutput(instance.Instance, id)
	value.AttachModuleOutput(instance)
}
