package renderer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lodastack/event/config"
	"github.com/lodastack/log"
)

const (
	PhantomjsBin = "phantomjs"
	RenderScript = "render.js"
)

type RenderOps struct {
	ID          string
	Ns          string
	Measurement string
	Time        time.Time
	Fn          string
	Title       string
	Where       string
	// Width  string
	// Height string
	// Timeout string
}

func RenderToPng(params *RenderOps) (string, error) {

	binPath, _ := filepath.Abs(filepath.Join(config.GetConfig().Render.PhantomDir, PhantomjsBin))
	renderScript, _ := filepath.Abs(filepath.Join(config.GetConfig().Render.PhantomDir, RenderScript))
	pngPath, _ := filepath.Abs(filepath.Join(config.GetConfig().Render.ImgDir, params.ID))
	pngPath = pngPath + ".png"

	renderUrl := fmt.Sprintf("%s?ns=%s&measurement=%s&starttime=%d&endtime=%d&fn=%s&title=%s&where=%s",
		config.GetConfig().Render.RenderUrl, params.Ns, params.Measurement,
		params.Time.Add(-60*time.Minute).Unix()*1000, params.Time.Unix()*1000,
		params.Fn, params.Title, params.Where)
	cmdArgs := []string{
		"--ignore-ssl-errors=true",
		"--proxy-type=none",
		renderScript,
		"png=" + pngPath,
		"url=" + renderUrl,
		"width=1000",
		"height=500",
	}

	cmd := exec.Command(binPath, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("RenderToPng fail:", err.Error())
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Error("RenderToPng fail:", err.Error())
		return "", err
	}
	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stdout, stderr)

	done := make(chan error)
	err = cmd.Start()
	if err != nil {
		log.Error("start RenderToPng fail")
		return "", err
	}
	go func() {
		defer close(done)
		err := cmd.Wait()
		if err != nil {
			done <- err
		}
	}()

	timeout := 15 // TODO
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		log.Errorf("renderToPng timeout (>%ds)", timeout)
		if err := cmd.Process.Kill(); err != nil {
			log.Error("failed to kill", "error", err)
		}
		return "", fmt.Errorf("renderToPng timeout (>%ds)", timeout)
	case err := <-done:
		if err != nil {
			log.Errorf("renderToPng fail: %s", err)
			return "", err
		}
	}

	log.Debug("Image rendered", "path", pngPath)
	return pngPath, nil
}
