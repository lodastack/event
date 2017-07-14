package mail

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/lodastack/event/models"
	"github.com/lodastack/event/renderer"
)

func getPngPath(alertMsg models.AlertMsg) (string, error) {
	var whereSql, whereStr string
	//where=("host"='xxx') AND ("interface"='xxx')

	for k, v := range alertMsg.Tags {
		whereSql = fmt.Sprintf(" (\"%s\"__'%s') AND %s", k, v, whereSql)
		whereStr = fmt.Sprintf("%s %s: %s", whereStr, k, v)
	}
	whereSql = strings.TrimRight(whereSql, "AND ")

	ID := fmt.Sprintf("%s-%s-%s-%s",
		alertMsg.Ns, alertMsg.Measurement, strings.Replace(whereSql, " ", "", -1), alertMsg.Time.Format("2006-01-02T15:04:05"))

	params := renderer.RenderOps{
		ID:          ID,
		Ns:          "collect." + alertMsg.Ns,
		Measurement: alertMsg.Measurement,
		Time:        time.Now(), // TODO: or alertMsg.Time?
		Fn:          "mean",
		Title:       alertMsg.Ns + " " + alertMsg.Measurement + whereStr,
		Where:       whereSql,
	}
	file, err := renderer.RenderToPng(&params)
	return file, err
}

func readPngToBase64(path string) ([]byte, error) {
	pngByte, err := ioutil.ReadFile(path)
	if err != nil || len(pngByte) == 0 {
		return nil, fmt.Errorf("invalid png file")
	}
	pngBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(pngByte)))
	base64.StdEncoding.Encode(pngBase64, pngByte)
	return pngBase64, nil
}

func getPngBase64(alertMsg models.AlertMsg) ([]byte, error) {
	filePath, err := getPngPath(alertMsg)
	if err != nil {
		return nil, err
	}
	return readPngToBase64(filePath)
}
