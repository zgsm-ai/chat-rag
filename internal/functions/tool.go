package functions

import "github.com/zgsm-ai/chat-rag/internal/types"

type ToolType string

const (
	ToolTypeServer ToolType = "server"
	ToolTypeIDE    ToolType = "ide"
)

type Parameter struct {
	Name        string      `yaml:"name" json:"name"`
	Type        string      `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Required    bool        `yaml:"required" json:"required"`
	Default     interface{} `yaml:"default" json:"default"`
	In          string      `yaml:"in" json:"in"`       // query/path/header
	Items       *Parameter  `yaml:"items" json:"items"` // for array type
	Enum        []string    `yaml:"enum" json:"enum"`
}

type Tool struct {
	Name        string      `yaml:"name" json:"name"`
	Type        ToolType    `yaml:"type" json:"type"`
	Description string      `yaml:"description" json:"description"`
	Endpoint    string      `yaml:"endpoint" json:"endpoint"`
	Method      string      `yaml:"method" json:"method"`
	Auth        *AuthConfig `yaml:"auth" json:"auth"`
	Parameters  []Parameter `yaml:"parameters" json:"parameters"`
}

type AuthConfig struct {
	Type     string `yaml:"type" json:"type"`
	Location string `yaml:"location" json:"location"`
}

func (t *Tool) ToFunctionDefinition() types.Function {
	props := make(map[string]types.PropertyDetails)
	var required []string

	for _, param := range t.Parameters {
		prop := types.PropertyDetails{
			Type:        param.Type,
			Description: param.Description,
			Default:     param.Default,
		}

		if param.Type == "array" && param.Items != nil {
			prop.Items = &types.Items{Type: param.Items.Type}
		}

		props[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	return types.Function{
		Type: "function",
		Function: types.FunctionDefinition{
			Name:        t.Name,
			Description: t.Description,
			Parameters: types.FunctionParameters{
				Type:       "object",
				Properties: props,
				Required:   required,
			},
		},
	}
}
