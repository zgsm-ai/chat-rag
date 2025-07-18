package functions

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

// FunctionCall is the structure of the function called by the LLM.
type FunctionDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Parameters  FunctionParameters `json:"parameters"`
}

type FunctionParameters struct {
	Type       string                     `json:"type"`
	Properties map[string]PropertyDetails `json:"properties"`
	Required   []string                   `json:"required"`
}

type PropertyDetails struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Items       *Items      `json:"items,omitempty"` // 用于array类型
}

type Items struct {
	Type string `json:"type"`
}

func (t *Tool) ToFunctionDefinition() FunctionDefinition {
	props := make(map[string]PropertyDetails)
	var required []string

	for _, param := range t.Parameters {
		prop := PropertyDetails{
			Type:        param.Type,
			Description: param.Description,
			Default:     param.Default,
		}

		if param.Type == "array" && param.Items != nil {
			prop.Items = &Items{Type: param.Items.Type}
		}

		props[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	return FunctionDefinition{
		Name:        t.Name,
		Description: t.Description,
		Parameters: FunctionParameters{
			Type:       "object",
			Properties: props,
			Required:   required,
		},
	}
}
