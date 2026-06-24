package main

import (
	"encoding/json"
	"io/fs"
	"os"
)

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory is not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}

	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}

	var paths []string
	err = fs.WalkDir(os.DirFS(dir), dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		paths = append(paths, p)

		return nil
	})

	if err != nil {
		return "", err
	}

	result, err := json.Marshal(paths)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, lists files in the current directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}
