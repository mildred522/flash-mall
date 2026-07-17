package httpx

import (
	"encoding/json"

	"github.com/cloudwego/hertz/pkg/app"
)

func DecodeJSONBody(c *app.RequestContext, destination any) error {
	body, err := c.Body()
	if err != nil {
		return err
	}
	return json.Unmarshal(body, destination)
}
