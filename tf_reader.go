package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// file loading
func loadModule(terraformRoot string, moduleRoot string, awsResources []Resource, state *HierarchyState) error {
	modulePath := filepath.Join(terraformRoot, moduleRoot)
	log.Debug("loading module: ", modulePath)

	module := state.NewModule(getModuleName(terraformRoot, moduleRoot))

	files, err := ioutil.ReadDir(modulePath)
	if err != nil {
		return fmt.Errorf("error reading directory: ", err)
	}

	for _, file := range files {

		if file.IsDir() {
			log.Debug(file.Name())

			log.Info("load module = ", filepath.Join(moduleRoot, file.Name()))
			err := loadModule(*rootDir, filepath.Join(moduleRoot, file.Name()), awsResources, state)

			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", file.Name(), err)
			}
		} else {
			moduleFile := filepath.Join(modulePath, file.Name())
			log.Info("moduleFile = ", moduleFile)
			_, err = loadModuleFile(module, moduleFile, awsResources, state)
			if err != nil {
				log.Errorf("error reading file '%s' (SKIPPED): %v", moduleFile, err)
			}
		}
	}
	return nil
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

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// process one of file root objects
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
		moduleInput := state.NewInput(module, VariableID(unquote(strKeys[1])))
		moduleInput.IsLoaded = true
	case "output":
		moduleOutput := state.NewOutput(module, VariableID(unquote(strKeys[1])))
		moduleOutput.IsLoaded = true
		processOutput(module, object.Val.(*ast.ObjectType), Map(strKeys[1:], unquote), awsResources, state)
	case "resource":
		processResource(module, object.Val.(*ast.ObjectType), Map(strKeys[1:], unquote), awsResources, state)
	case "module":
		processModule(module, object.Val.(*ast.ObjectType), Map(strKeys[1:], unquote), awsResources, state)
	default:
		log.Warning("process module object: unknown item type: ", strKeys[0])
	}

	return state, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// process resource
func processResource(module *Module, object *ast.ObjectType, resourceName []string, awsResources []Resource, state *HierarchyState) {
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
				findInputVariableAsArgumentUsages(value.Token.Text, module, fieldResourceName, awsResources, state)
				//findModuleOutputUsages(value.Token.Text, module, fieldResourceName, awsResources, state)
			default:
				log.Warningf("process resource: unsupported value type for resourceName: %v value: %+v", fieldResourceName, value)
			}
		}
	}
}

func findModuleOutputUsages(token string, module *Module, fieldResourceName []string, awsResources []Resource, state *HierarchyState) {
	log.Fatal("NOT IMPLEMENTED!")
}

func findInputVariableAsArgumentUsages(token string, module *Module, fieldResourceName []string, awsResources []Resource, state *HierarchyState) {
	variableUsages := findAllVariables(token)

	resourceName := fieldResourceName[0]
	resourceFieldName := fieldResourceName[2]
	for i := 0; i < len(variableUsages); i++ {
		variableName := variableUsages[i]
		awsArgument := getArgumentByName(resourceName, resourceFieldName, awsResources)
		state.ConnectInputToArgument(module, variableName, fieldResourceName, awsArgument)
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// process module instance
func processModule(module *Module, object *ast.ObjectType, resourceName []string, awsResources []Resource, state *HierarchyState) {
	instanceName := resourceName[0]
	module.NewInstance(instanceName, module)
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
				findInputVariableModuleInputUsages(value.Token.Text, module, fieldResourceName, awsResources, state)
				//findModuleOutputUsages(value.Token.Text, module, fieldResourceName, awsResources, state)
			default:
				log.Warningf("process resource: unsupported value type for resourceName: %v value: %+v", fieldResourceName, value)
			}
		}
	}
}

func findInputVariableModuleInputUsages(token string, module *Module, fieldResourceName []string, awsResources []Resource, state *HierarchyState) {
	variableUsages := findAllVariables(token)

	for i := 0; i < len(variableUsages); i++ {

		variableName := variableUsages[i]
		if "" != variableName {
			moduleInstanceName := fieldResourceName[0]
			//awsArgument := getArgumentByName([]string{fieldResourceName[0], fieldResourceName[2]}, awsResources)
			moduleInstance := module.FindModuleInstance(moduleInstanceName)
			state.ConnectInputToModuleInput(module, variableName, fieldResourceName, moduleInstance)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// process module output
func processOutput(module *Module, object *ast.ObjectType, resourceName []string, awsResources []Resource, state *HierarchyState) {
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
				findModuleOutputValues(value.Token.Text, module, fieldResourceName, awsResources, state)
			default:
				log.Warningf("process resource: unsupported value type for resourceName: %v value: %+v", fieldResourceName, value)
			}
		}
	}
}

func findModuleOutputValues(token string, module *Module, fieldResourceName []string, awsResources []Resource, state *HierarchyState) {
	resourceFields := findAllResourceFields(token)
	moduleOutputName := fieldResourceName[1]

	for _, resourceField := range resourceFields {
		awsAttribute := getAttributeByName(resourceField.Name, resourceField.FieldName, awsResources)
		state.ConnectOutputToAttribute(module, VariableID(moduleOutputName), awsAttribute)
		log.Debugf("field resource name = %+v awsArgument = %+v", resourceField, awsAttribute)
	}

	moduleFields := findAllModuleFields(token)
	for _, moduleField := range moduleFields {
		//state.ConnectOutputToModuleOutput(module, VariableID(moduleOutputName), awsAttribute)
		log.Debugf("module output name = %+v", moduleField)
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// regex utilities

func findAllVariables(token string) []VariableID {
	re := regexp.MustCompile("var\\.([-a-zA-Z0-9_]*)")
	matches := re.FindAllStringSubmatch(token, -1)

	result := make([]VariableID, 0)
	for i := 0; i < len(matches); i++ {
		if len(matches[i]) == 2 {
			result = append(result, VariableID(matches[i][1]))
		}
	}

	return result
}

func findAllResourceFields(token string) []ResourceFieldID {
	re := regexp.MustCompile("(aws_[-a-zA-Z_0-9]*)\\.([-a-zA-Z_0-9]*)\\.([-a-zA-Z_0-9]*)")
	matches := re.FindAllStringSubmatch(token, -1)

	result := make([]ResourceFieldID, 0)
	for i := 0; i < len(matches); i++ {
		if len(matches[i]) == 4 {
			result = append(result, ResourceFieldID{Name: matches[i][1], InstanceName: matches[i][2], FieldName: matches[i][3]})
		}
	}

	return result
}

func findAllModuleFields(token string) []ModuleFieldID {
	re := regexp.MustCompile("module\\.([-a-zA-Z_0-9]*)\\.([-a-zA-Z_0-9]*)")
	matches := re.FindAllStringSubmatch(token, -1)

	result := make([]ModuleFieldID, 0)
	for i := 0; i < len(matches); i++ {
		if len(matches[i]) == 3 {
			result = append(result, ModuleFieldID{InstanceName: matches[i][1], FieldName: matches[i][2]})
		}
	}

	return result
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// aws resource description

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

func getArgumentByName(resourceName string, fieldName string, awsResources []Resource) *ResourceArgument {
	s1 := unquote(resourceName)
	s2 := unquote(fieldName)

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

func getAttributeByName(resourceName string, fieldName string, awsResources []Resource) *ResourceAttribute {
	s1 := unquote(resourceName)
	s2 := unquote(fieldName)

	for _, res := range awsResources {
		if res.Name == s1 {
			for _, arg := range res.Attributes {
				if arg.Name == s2 {
					return &arg
				}
			}
			break
		}
	}
	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
