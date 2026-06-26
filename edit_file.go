package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
)

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and musto nly have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)

	dirPerm := os.FileMode(0o755)

	if dir != "." {
		err := os.MkdirAll(dir, dirPerm)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %v", err)
		}
	}

	filePerm := os.FileMode(0o644)
	err := os.WriteFile(filePath, []byte(content), filePerm)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}

	return fmt.Sprintf("Successfully created file %v", filePath), nil
}

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)

	if err != nil {
		return "", err
	}

	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}

	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}

	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)

	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old_str not found in file")
	}

	err = os.WriteFile(editFileInput.Path, []byte(content), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	return "OK", nil
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.
	Replaces 'old_str with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.
	If the file specified with path doesn't exist, it will be created.`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}
