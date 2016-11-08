package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"

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

	_, err = loadModule(*rootDir, ".", awsResources, state)

	if err != nil {
		log.Errorf("error reading root module '%s' (SKIPPED): %v", *rootDir, err)
	}

	jsonState, err := json.Marshal(*state)
	if nil != err {
		log.Error(err)
	}
	log.Infof("result = %v", string(jsonState))
}

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

func loadModule(terraformRoot string, moduleRoot string, awsResources []Resource, state *HierarchyState) (*Module, error) {
	modulePath := filepath.Join(terraformRoot, moduleRoot)
	log.Debug("loading module: ", modulePath)

	module := state.NewModule(moduleRoot)

	files, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: ", err)
	}

	for _, file := range files {

		if file.IsDir() {
			log.Debug(file.Name())

			log.Info("load module = ", filepath.Join(moduleRoot, file.Name()))
			childModule, err := loadModule(*rootDir, filepath.Join(moduleRoot, file.Name()), awsResources, state)

			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", file.Name(), err)
			}

			module.Submodules = append(module.Submodules, childModule)
		} else {
			moduleFile := filepath.Join(modulePath, file.Name())
			log.Info("moduleFile = ", moduleFile)
			_, err = loadModuleFile(module, moduleFile, awsResources, state)
			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", moduleFile, err)
			}
		}
	}
	return module, nil
}

func loadModuleFile(module *Module, filePath string, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	re := regexp.MustCompile(".*\\.tf")
	if !re.MatchString(filePath) {
		return state, nil
	}

	log.Debug("module file loading: ", filePath)

	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("module file loading (%s): %v", filePath, err)
	}

	hclFile, err := hcl.Parse(string(bytes))
	if err != nil {
		return nil, fmt.Errorf("module file loading (%s): unmarshalling from hcl: %v", filePath, err)
	}

	objects := hclFile.Node.(*ast.ObjectList)

	for _, objItem := range objects.Items {
		_, err = processModuleObject(module, objItem, awsResources, state)
		if nil != err {
			log.Warningf("module file loading (%s): error processing module object: %v", filePath, err)
		}
	}

	log.Debugf("module file loading (%s): loaded module: %+v", filePath, module)

	return state, nil
}

func processModuleObject(module *Module, object *ast.ObjectItem, awsResources []Resource, state *HierarchyState) (*HierarchyState, error) {
	var strKeys []string

	for _, key := range object.Keys {
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
			fieldResourceName := resourceName
			for _, k := range i.Keys {
				fieldResourceName = append(fieldResourceName, k.Token.Text)
			}

			switch value := i.Val.(type) {
			case *ast.LiteralType:
				log.Debug("Value = ", value.Token.Text)
				re := regexp.MustCompile("\\${var\\.([-a-zA-Z_]*)}")

				attributesMatched := re.FindStringSubmatch(value.Token.Text)
				if len(attributesMatched) != 2 {
					log.Errorf("process resource: wrong number of matches of regexp. match: %v", attributesMatched)
					continue
				}

				variableName := attributesMatched[1]

				if "" != variableName {
					awsArgument := getArgumentByName([]string{fieldResourceName[0], fieldResourceName[2]}, awsResources)

					state.ConnectInputToArgument(module, variableName, fieldResourceName, awsArgument)
				}
			default:
				log.Warningf("process resource: unsupported value type for resourceName: %v value: %+v", fieldResourceName, value)
			}
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
		return nil
	}
	log.Debug("argName = ", argName)

	s1, err := strconv.Unquote(argName[0])
	if err != nil {
		s1 = argName[0]
	}

	s2, err := strconv.Unquote(argName[1])
	if err != nil {
		s2 = argName[1]
	}

	for _, res := range awsResources {
		if res.Name == s1 {
			for _, arg := range res.Arguments {
				if arg.Name == s2 {
					return &arg
				}
			}
			break
		}
	}
	return nil
}
