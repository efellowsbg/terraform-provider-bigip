package bigip

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
)

type ILXWorkspace struct {
	Name            string      `json:"name,omitempty"`
	FullPath        string      `json:"fullPath,omitempty"`
	Generation      int         `json:"generation,omitempty"`
	SelfLink        string      `json:"selfLink,omitempty"`
	NodeVersion     string      `json:"nodeVersion,omitempty"`
	StagedDirectory string      `json:"stagedDirectory,omitempty"`
	Version         string      `json:"version,omitempty"`
	Extensions      []Extension `json:"extensions,omitempty"`
	Rules           []File      `json:"rules,omitempty"`
}

type File struct {
	Name string `json:"name,omitempty"`
}

type Extension struct {
	Name  string `json:"name,omitempty"`
	Files []File `json:"files,omitempty"`
}

func (b *BigIP) GetWorkspace(ctx context.Context, path string) (*ILXWorkspace, error) {
	spc := &ILXWorkspace{}
	err, exists := b.getForEntity(spc, uriMgmt, uriTm, uriIlx, uriWorkspace, path)
	if !exists {
		return nil, fmt.Errorf("workspace does not exist: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting ILX Workspace: %w", err)
	}

	return spc, nil
}

func (b *BigIP) CreateWorkspace(ctx context.Context, path string) error {
	err := b.post(ILXWorkspace{Name: path}, uriMgmt, uriTm, uriIlx, uriWorkspace, "")
	if err != nil {
		return fmt.Errorf("error creating ILX Workspace: %w", err)
	}

	return nil
}

func (b *BigIP) DeleteWorkspace(ctx context.Context, name string) error {
	err := b.delete(uriMgmt, uriTm, uriIlx, uriWorkspace, name)
	if err != nil {
		return fmt.Errorf("error deleting ILX Workspace: %w", err)
	}
	return nil
}

func (b *BigIP) PatchWorkspace(ctx context.Context, name string) error {
	err := b.patch(ILXWorkspace{Name: name}, uriMgmt, uriTm, uriIlx, uriWorkspace, name)
	if err != nil {
		return fmt.Errorf("error patching ILX Workspace: %w", err)
	}
	return nil
}

type ExtensionConfig struct {
	Name          string `json:"name,omitempty"`
	Partition     string `json:"partition,omitempty"`
	WorkspaceName string `json:"workspaceName,omitempty"`
}

func (b *BigIP) CreateExtension(ctx context.Context, opts ExtensionConfig) error {
	err := b.post(ILXWorkspace{Name: opts.WorkspaceName}, uriMgmt, uriTm, uriIlx, uriWorkspace+"?options=extension,"+opts.Name)
	if err != nil {
		return fmt.Errorf("error creating ILX Extension: %w", err)
	}
	return nil
}

// UploadExtensionFiles uploads the files in the given directory to the BIG-IP system
// Only index.js and package.json files are uploaded as they are the only mutable files.
func (b *BigIP) UploadExtensionFiles(ctx context.Context, opts ExtensionConfig, path string) error {
	destination := fmt.Sprintf("%s/%s/%s/extensions/%s/", WORKSPACE_UPLOAD_PATH, opts.Partition, opts.WorkspaceName, opts.Name)
	files, err := readFilesFromDirectory(path)
	if err != nil {
		return err
	}
	err = b.uploadFilesToDestination(files, destination)
	if err != nil {
		return err
	}
	return nil
}

func (b *BigIP) UploadRuleFiles(ctx context.Context, opts ExtensionConfig, path string) error {
	destination := fmt.Sprintf("%s/%s/%s/rules/", WORKSPACE_UPLOAD_PATH, opts.Partition, opts.WorkspaceName)
	files, err := readFilesFromDirectory(path)
	if err != nil {
		return err
	}
	if err = b.uploadFilesToDestination(files, destination); err != nil {
		return err
	}
	return nil
}

func (b *BigIP) uploadFilesToDestination(files []*os.File, destination string) error {
	uploadedFilePaths, err := b.uploadFiles(files)
	if err != nil {
		return err
	}
	for _, uploadedFilePath := range uploadedFilePaths {
		err := b.runCatCommand(uploadedFilePath, destination)
		if err != nil {
			return err
		}
	}
	return nil
}

func readFilesFromDirectory(path string) ([]*os.File, error) {
	fileDirs, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w", err)
	}
	files := []*os.File{}
	for _, fileDir := range fileDirs {
		if fileDir.IsDir() {
			continue
		}
		f, err := fileFromDirEntry(fileDir, path)
		if err != nil {
			return nil, fmt.Errorf("error getting file from directory entry: %w", err)
		}
		files = append(files, f)
	}
	return files, nil
}

func (b *BigIP) uploadFiles(files []*os.File) ([]string, error) {
	uploadedFilePaths := []string{}
	for _, file := range files {
		if file.Name() == "index.js" || file.Name() == "package.json" {
			res, err := b.UploadFile(file)
			if err != nil {
				return nil, fmt.Errorf("error uploading file: %w", err)
			}
			uploadedFilePaths = append(uploadedFilePaths, res.LocalFilePath)
		}
	}
	return uploadedFilePaths, nil
}

func (b *BigIP) runCatCommand(uploadedFilePath, destination string) error {
	fileName := filepath.Base(uploadedFilePath)
	command := BigipCommand{
		Command:     "run",
		UtilCmdArgs: fmt.Sprintf("-c 'cat %s > %s'", uploadedFilePath, destination+fileName),
	}
	output, err := b.RunCommand(&command)
	if err != nil {
		return fmt.Errorf("error running command: %w", err)
	}
	spew.Dump(output)
	return nil
}

func fileFromDirEntry(entry fs.DirEntry, dir string) (*os.File, error) {
	path := filepath.Join(dir, entry.Name())

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return file, nil
}
