package main

import (
	log "github.com/Sirupsen/logrus"
)

type ResourceArgumentUsage struct {
	Arg       *ResourceArgument `form:"Arg" json:"Arg" xml:"Arg"`
	UsagePath []string          `form:"UsagePath" json:"UsagePath" xml:"UsagePath"`
}

type ModuleInputUsage struct {
	Input     *ResourceArgument `form:"Input" json:"Input" xml:"Input"`
	UsagePath []string          `form:"UsagePath" json:"UsagePath" xml:"UsagePath"`
}

type ModuleInput struct {
	Name          string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded      bool
	AsArgument    []ResourceArgumentUsage `form:"AsArgument" json:"AsArgument" xml:"AsArgument"`
	AsModuleInput []ModuleInputUsage      `form:"AsModuleInput" json:"AsModuleInput" xml:"AsModuleInput"`
}

type ModuleOutput struct {
	Name             string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded         bool
	FromAttribute    []*ResourceAttribute `form:"FromAttribute" json:"FromAttribute" xml:"FromAttribute"`
	FromModuleOutput []*ModuleOutput      `form:"FromModuleOutput" json:"FromModuleOutput" xml:"FromModuleOutput"`
}

type Module struct {
	Name       string `form:"Name" json:"Name" xml:"Name"`
	IsLoaded   bool
	Submodules []*Module
	Inputs     []*ModuleInput  `form:"Inputs" json:"Inputs" xml:"Inputs"`
	Outputs    []*ModuleOutput `form:"Outputs" json:"Outputs" xml:"Outputs"`
}

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

func (h *HierarchyState) NewModule(name string) *Module {
	m, found := h.allModulesMap[name]
	if !found {
		h.AllModules = append(h.AllModules, Module{Name: name, IsLoaded: false})
		m = &h.AllModules[len(h.AllModules)-1]
		h.allModulesMap[name] = m
	}
	return m
}

func (h *HierarchyState) NewInput(module *Module, name string) *ModuleInput {
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

	m.AsArgument = append(m.AsArgument, ResourceArgumentUsage{Arg: argument, UsagePath: usagePath})
}

func (h *HierarchyState) ConnectInputToArgument(module *Module, name string, usagePath []string, argument *ResourceArgument) {

	log.Debugf("module %v name %v attach argument %v", module.Name, name, argument)

	value := h.NewInput(module, name)
	value.AttachArgument(usagePath, argument)
}

func (h *HierarchyState) NewOutput(module *Module, name string) *ModuleOutput {
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
