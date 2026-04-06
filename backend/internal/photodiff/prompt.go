package photodiff

// PromptVersion identifies the version of the comparison prompt template.
const PromptVersion = "v1"

// PhotoDiffPromptData provides template variables for the photo diff prompt.
type PhotoDiffPromptData struct {
	PairsCount int
}
