package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/vharitonsky/iniflags"
)

var (
	rootDir         = flag.String("dir", ".", "start dir")
	descriptionPath = flag.String("desc", "", "terraform markdown description")
	outPath         = flag.String("out", "out.json", "output result filepath")
)

type Line struct {
	Name        string `form:"Name" json:"Name" xml:"Name"`
	Optional    bool   `form:"Optional" json:"Optional" xml:"Optional"`
	Description string `form:"Description" json:"Description" xml:"Description"`
}

type ResourceArgument Line
type ResourceAttribute Line

type Resource struct {
	Name       string              `form:"Name" json:"Name" xml:"Name"`
	Arguments  []ResourceArgument  `form:"Arguments" json:"Arguments" xml:"Arguments"`
	Attributes []ResourceAttribute `form:"Attributes" json:"Attributes" xml:"Attributes"`
}

func main() {
	iniflags.Parse()
	log.SetLevel(log.DebugLevel)
	log.Debug("reading directory: ", *rootDir)

	awsResources, err := loadResources(*descriptionPath)
	if err != nil {
		log.Error("error loading aws resources: ", err)
		return
	}

	state := NewHierarchyState()

	files, err := ioutil.ReadDir(*rootDir)
	if err != nil {
		log.Error("error reading directory: ", err)
		return
	}

	for _, file := range files {
		state, err = loadModule(*rootDir, file.Name(), awsResources, state)

		if err != nil {
			log.Errorf("error reading file '%s' (SKIPPED): %v", file.Name(), err)
		}
	}
}

type ModuleInput struct {
	Name          string
	IsLoaded      bool
	AsArgument    []*ResourceArgument
	AsModuleInput []*ModuleInput
}

type ModuleOutput struct {
	Name             string
	IsLoaded         bool
	FromAttribute    []*ResourceAttribute
	FromModuleOutput []*ModuleOutput
}

type Module struct {
	Name     string
	IsLoaded bool
	Inputs   []*ModuleInput
	Outputs  []*ModuleOutput
}

type HierarchyState struct {
	AllModules []Module
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

func loadModule(moduleRoot string, path string, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	modulePath := filepath.Join(moduleRoot, path)
	log.Debug("loading module: ", modulePath)

	module := state.NewModule(moduleRoot)

	bytes, err := ioutil.ReadFile(modulePath)
	if err != nil {
		return nil, fmt.Errorf("module loading: %v", err)
	}

	hclFile, err := hcl.Parse(string(bytes))
	if err != nil {
		return nil, fmt.Errorf("module loading: unmarshalling from hcl: %v", err)
	}

	objects := hclFile.Node.(*ast.ObjectList)

	for _, objItem := range objects.Items {
		_, err = processModuleObject(module, objItem, awsResources, state)
		if nil != err {
			log.Warning("module loading: error processing module object: ", err)
		}
	}

	log.Debugf("module loading: loaded module: %+v", module)

	return nil, nil
}

func processModuleObject(module *Module, object *ast.ObjectItem, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	var strKeys []string

	log.Debug("KEYS")
	for _, key := range object.Keys {
		log.Debug(key.Token.Text)
		strKeys = append(strKeys, key.Token.Text)
	}

	if len(strKeys) < 2 {
		return nil, fmt.Errorf("process module object: wrong number of object keys (expected at least 2)")
	}

	switch strKeys[0] {
	case "variable":
		moduleInput := state.NewInput(module, strKeys[1])
		moduleInput.IsLoaded = true
		log.Debug(moduleInput)
	case "output":
		moduleOutput := state.NewOutput(module, strKeys[1])
		moduleOutput.IsLoaded = true
		log.Debug(moduleOutput)
	case "resource":
		processResource(module, object.Val.(*ast.ObjectType), strKeys[1:], awsResources, state)
	case "module":
		//module instantiation
	default:
		log.Warning("process module object: unknown item type: ", strKeys[0])
	}

	return state, nil
}

func processResource(module *Module, object *ast.ObjectType, resourceName []string, awsResources []Resource, state *HierarchyState) {
	log.Debug("resource name = ", resourceName)

	if nil != object.List && nil != object.List.Items {
		for _, i := range object.List.Items {
			if len(i.Keys) != 1 {
				log.Error("process resource: wrong number of keys, expected 1")
				return
			}
			for _, k := range i.Keys {
				resourceName = append(resourceName, k.Token.Text)
			}
			awsArgument := getArgumentByName([]string{resourceName[0], resourceName[2]}, awsResources)
			log.Debug("\t\t argument = ", awsArgument)

			log.Debug("\t\t\tNODE = ", i.Val)
		}
	}
}

func loadResources(path string) ([]Resource, error) {
	var resources []Resource
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("resource loading: %v", err)
	}

	err = json.Unmarshal(bytes, &resources)
	if err != nil {
		return nil, fmt.Errorf("resource loading: error unmarshalling resources: %v", err)
	}

	return resources, nil
}

func getArgumentByName(argName []string, awsResources []Resource) *ResourceArgument {
	if len(argName) < 2 {
		log.Error("get argument by name: too short name")
	}

	log.Debug("argName = ", argName)

	for _, res := range awsResources {
		if res.Name == argName[0] {
			for _, arg := range res.Arguments {
				if arg.Name == argName[1] {
					return &arg
				}
			}
			break
		}
	}
	return nil
}
