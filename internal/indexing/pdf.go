package indexing

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rpg-helper-bot/rpg-helper-bot/internal/textutil"
)

func ExtractPages(pdfPath string, startPage, endPage int) (string, error) {
	if startPage < 1 || endPage < startPage {
		return "", fmt.Errorf("invalid page range %d-%d", startPage, endPage)
	}
	cmd := exec.Command("pdftotext",
		"-f", strconv.Itoa(startPage),
		"-l", strconv.Itoa(endPage),
		pdfPath,
		"-",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return textutil.NormalizePDFText(strings.TrimSpace(stdout.String())), nil
}

func PageCount(pdfPath string) (int, error) {
	cmd := exec.Command("pdfinfo", pdfPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.Atoi(fields[1])
			}
		}
	}
	return 0, fmt.Errorf("could not parse page count from pdfinfo")
}

func RenderThumbnail(pdfPath, outPNG string, maxPx int) error {
	if err := os.MkdirAll(filepath.Dir(outPNG), 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(outPNG), "thumb-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	prefix := filepath.Join(tmpDir, "page")
	cmd := exec.Command("pdftoppm",
		"-png",
		"-f", "1",
		"-l", "1",
		"-scale-to", strconv.Itoa(maxPx),
		pdfPath,
		prefix,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdftoppm: %w: %s", err, strings.TrimSpace(string(out)))
	}

	matches, err := filepath.Glob(prefix + "*.png")
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("pdftoppm: no output png found")
	}
	return os.Rename(matches[0], outPNG)
}

func ToolsAvailable() error {
	for _, tool := range []string{"pdftotext", "pdftoppm", "pdfinfo"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s not found in PATH (install poppler)", tool)
		}
	}
	return nil
}
