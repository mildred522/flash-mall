package httpx

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestDecodeJSONBodyRejectsMalformedJSON(t *testing.T) {
	request := &app.RequestContext{}
	request.Request.SetBodyString(`{"product_id":`)
	var body struct {
		ProductID int64 `json:"product_id"`
	}
	if err := DecodeJSONBody(request, &body); err == nil {
		t.Fatal("malformed JSON must be rejected")
	}
}
