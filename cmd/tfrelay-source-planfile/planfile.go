package main

import (
	"archive/zip"
	"encoding/json"
	"io"

	"github.com/kamichidu/go-tfrelay/internal/planproto"
	"google.golang.org/protobuf/proto"
)

type planfile struct {
	reader *zip.ReadCloser
}

var (
	_ io.Closer = (*planfile)(nil)
)

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
