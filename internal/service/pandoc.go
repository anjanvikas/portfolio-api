package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrConverterUnavailable is returned when the conversion tool (pandoc) is not
// installed on the host. The handler maps it to a 503 so the admin sees an
// actionable "install pandoc" error rather than a generic failure.
var ErrConverterUnavailable = errors.New("docx converter unavailable")

// DocxConverter turns the bytes of a .docx file into a GitHub-flavored markdown
// string. Declared as an interface so the handler is unit-testable with a fake,
// matching the pattern the rest of the package follows.
type DocxConverter interface {
	ConvertDocx(ctx context.Context, docx []byte) (string, error)
}

// PandocConverter shells out to the pandoc binary. The uploaded file is written
// to a private temp file (pandoc reads docx — a zip container — from a seekable
// path, not a stream), converted, then deleted: the document is never persisted
// beyond the conversion, only the resulting text is kept.
type PandocConverter struct {
	// binary is the pandoc executable name/path; overridable in tests.
	binary string
}

// NewPandocConverter returns a converter that invokes the "pandoc" binary found
// on PATH.
func NewPandocConverter() *PandocConverter {
	return &PandocConverter{binary: "pandoc"}
}

// ConvertDocx writes the docx bytes to a temp file, runs
// `pandoc --from=docx --to=gfm`, and returns stdout. The temp file is removed
// before returning regardless of outcome.
func (c *PandocConverter) ConvertDocx(ctx context.Context, docx []byte) (string, error) {
	if _, err := exec.LookPath(c.binary); err != nil {
		return "", ErrConverterUnavailable
	}

	tmp, err := os.CreateTemp("", "upload-*.docx")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(docx); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	// --wrap=none keeps paragraphs on single lines so the markdown editor's
	// preview wraps them naturally instead of inheriting pandoc's hard wraps.
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.binary, "--from=docx", "--to=gfm", "--wrap=none", tmpPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pandoc conversion failed: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
