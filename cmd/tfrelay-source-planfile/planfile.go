package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/kamichidu/go-tfrelay/internal/planproto"
	"google.golang.org/protobuf/proto"
)

type planfile struct {
	reader *zip.ReadCloser
}

var (
	_ io.Closer = (*planfile)(nil)
)

func createPlanfile(command string, args ...string) (string, error) {
	log.Printf("info: create planfile by %v %v", command, args)
	tmpfile, err := os.CreateTemp("", "tfplan.*")
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()
	log.Printf("info: planfile will save at %v", tmpfile.Name())

	var cmdArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] != "--out" {
			cmdArgs = append(cmdArgs, args[i])
			continue
		}
		i = i + 1
	}
	cmdArgs = append(cmdArgs, "--out", tmpfile.Name())

	cmd := exec.Command(command, cmdArgs...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func openPlanfile(name string) (*planfile, error) {
	zr, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	return &planfile{
		reader: zr,
	}, nil
}

func (f *planfile) GetTfPlan() (*planproto.Plan, error) {
	b, err := f.readFile("tfplan")
	if err != nil {
		return nil, err
	}
	var out planproto.Plan
	if err := proto.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (f *planfile) GetTfState() (tfState, error) {
	b, err := f.readFile("tfstate")
	if err != nil {
		return tfState{}, err
	}
	var out tfState
	if err := json.Unmarshal(b, &out); err != nil {
		return tfState{}, err
	}
	return out, nil
}

func (f *planfile) readFile(name string) ([]byte, error) {
	zf, err := f.reader.Open(name)
	if err != nil {
		return nil, err
	}
	defer zf.Close()
	return io.ReadAll(zf)
}

func (f *planfile) Close() error {
	return f.reader.Close()
}
