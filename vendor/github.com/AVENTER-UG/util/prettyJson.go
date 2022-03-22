package util

import (
	"bytes"
	"encoding/json"
)

func PrettyJSON(data []byte) string {
	var pretty bytes.Buffer
	json.Indent(&pretty, data, "", "\t")
	return string(pretty.Bytes())
}
