package mail

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/lodastack/event/models"
	"github.com/lodastack/event/renderer"
	"github.com/lodastack/log"
)

func getPngPath(notifyData models.NotifyData) (string, error) {
	var whereSql, whereStr string
	//where=("host"='xxx') AND ("interface"='xxx')

	for k, v := range notifyData.Tags {
		whereSql = fmt.Sprintf(" (\"%s\"__'%s') AND %s", k, v, whereSql)
		whereStr = fmt.Sprintf("%s %s: %s", whereStr, k, v)
	}
	whereSql = strings.TrimRight(whereSql, "AND ")

	ID := fmt.Sprintf("%s-%s-%s-%s",
		notifyData.Ns, notifyData.Measurement, strings.Replace(whereSql, " ", "", -1), notifyData.Time.Format("2006-01-02T15:04:05"))

	params := renderer.RenderOps{
		ID:          ID,
		Ns:          "collect." + notifyData.Ns,
		Measurement: notifyData.Measurement,
		Time:        time.Now(), // TODO: or alertMsg.Time?
		Fn:          "mean",
		Title:       notifyData.Ns + " " + notifyData.Measurement + whereStr,
		Where:       whereSql,
	}
	return renderer.RenderToPng(&params)
}

func readPngToBase64(path string) ([]byte, error) {
	pngByte, err := ioutil.ReadFile(path)
	if err != nil || len(pngByte) == 0 {
		log.Errorf("readPngToBase64 fail: err: %v, length: %d", err, len(pngByte))
		return nil, fmt.Errorf("invalid png file")
	}
	pngBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(pngByte)))
	base64.StdEncoding.Encode(pngBase64, pngByte)
	return pngBase64, nil
}

func getPngBase64(notifyData models.NotifyData) ([]byte, error) {
	filePath, err := getPngPath(notifyData)
	if err != nil {
		return nil, err
	}
	return readPngToBase64(filePath)
}
